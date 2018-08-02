package containerstore

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

const (
	CredCreationSucceededCount    = "CredCreationSucceededCount"
	CredCreationSucceededDuration = "CredCreationSucceededDuration"
	CredCreationFailedCount       = "CredCreationFailedCount"
)

type Credential struct {
	Cert string
	Key  string
}

//go:generate counterfeiter -o containerstorefakes/fake_cred_manager.go . CredManager
type CredManager interface {
	CreateCredDir(lager.Logger, executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error)
	RemoveCredDir(lager.Logger, executor.Container) error
	Runner(lager.Logger, executor.Container) ifrit.Runner
}

type noopManager struct{}

func NewNoopCredManager() CredManager {
	return &noopManager{}
}

func (c *noopManager) CreateCredDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error) {
	return nil, nil, nil
}

func (c *noopManager) RemoveCredDir(logger lager.Logger, container executor.Container) error {
	return nil
}

func (c *noopManager) Runner(lager.Logger, executor.Container) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)
		<-signals
		return nil
	})
}

type credManager struct {
	logger         lager.Logger
	metronClient   loggingclient.IngressClient
	validityPeriod time.Duration
	entropyReader  io.Reader
	clock          clock.Clock
	CaCert         *x509.Certificate
	privateKey     *rsa.PrivateKey
	handlers       []CredentialHandler
}

//go:generate counterfeiter -o containerstorefakes/fake_cred_handler.go . CredentialHandler

// CredentialHandler handles new credential generated by the CredManager.
type CredentialHandler interface {
	// Called to create the necessary directory
	CreateDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error)

	// Called during shutdown to remove directory created in CreateDir
	RemoveDir(logger lager.Logger, container executor.Container) error

	// Called periodically as new valid certificate/key pair are generated
	Update(credentials Credential, container executor.Container) error

	// Called when the CredManager is preparing to exit. This is mainly to update
	// the EnvoyProxy with invalid certificates and prevent it from accepting
	// more incoming traffic from the gorouter
	Close(invalidCredentials Credential, container executor.Container) error
}

func NewCredManager(
	logger lager.Logger,
	metronClient loggingclient.IngressClient,
	validityPeriod time.Duration,
	entropyReader io.Reader,
	clock clock.Clock,
	CaCert *x509.Certificate,
	privateKey *rsa.PrivateKey,
	handlers ...CredentialHandler,
) CredManager {
	return &credManager{
		logger:         logger,
		metronClient:   metronClient,
		validityPeriod: validityPeriod,
		entropyReader:  entropyReader,
		clock:          clock,
		CaCert:         CaCert,
		privateKey:     privateKey,
		handlers:       handlers,
	}
}

func calculateCredentialRotationPeriod(validityPeriod time.Duration) time.Duration {
	if validityPeriod > 4*time.Hour {
		return validityPeriod - 30*time.Minute
	}

	eighth := validityPeriod / 8
	return validityPeriod - eighth
}

func (c *credManager) CreateCredDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error) {
	var mounts []garden.BindMount
	var envs []executor.EnvironmentVariable
	for _, h := range c.handlers {
		handlerMounts, handlerEnv, err := h.CreateDir(logger, container)
		if err != nil {
			return nil, nil, err
		}
		envs = append(envs, handlerEnv...)
		mounts = append(mounts, handlerMounts...)
	}
	return mounts, envs, nil
}

func (c *credManager) RemoveCredDir(logger lager.Logger, container executor.Container) error {
	err := &multierror.Error{ErrorFormat: func(errs []error) string {
		var s string
		for _, e := range errs {
			if e != nil {
				if s != "" {
					s += "; "
				}
				s += e.Error()
			}
		}
		return s
	}}

	for _, h := range c.handlers {
		handlerErr := h.RemoveDir(logger, container)
		err = multierror.Append(err, handlerErr)
	}
	return err
}

func (c *credManager) Runner(logger lager.Logger, container executor.Container) ifrit.Runner {
	runner := ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		logger = logger.Session("cred-manager-runner")
		logger.Info("starting")
		defer logger.Info("finished")

		start := c.clock.Now()
		creds, err := c.generateCreds(logger, container, container.Guid)
		duration := c.clock.Since(start)
		if err != nil {
			logger.Error("failed-to-generate-credentials", err)
			c.metronClient.IncrementCounter(CredCreationFailedCount)
			return err
		}

		for _, h := range c.handlers {
			err := h.Update(creds, container)
			if err != nil {
				return err
			}
		}

		c.metronClient.IncrementCounter(CredCreationSucceededCount)
		c.metronClient.SendDuration(CredCreationSucceededDuration, duration)

		rotationDuration := calculateCredentialRotationPeriod(c.validityPeriod)
		regenCertTimer := c.clock.NewTimer(rotationDuration)

		close(ready)
		logger.Info("started")

		regenLogger := logger.Session("regenerating-cert-and-key")
		for {
			select {
			case <-regenCertTimer.C():
				regenLogger.Debug("started")
				start := c.clock.Now()
				creds, err := c.generateCreds(logger, container, container.Guid)
				duration := c.clock.Since(start)
				if err != nil {
					regenLogger.Error("failed-to-generate-credentials", err)
					c.metronClient.IncrementCounter(CredCreationFailedCount)
					return err
				}
				c.metronClient.IncrementCounter(CredCreationSucceededCount)
				c.metronClient.SendDuration(CredCreationSucceededDuration, duration)

				for _, h := range c.handlers {
					err := h.Update(creds, container)
					if err != nil {
						return err
					}
				}

				rotationDuration = calculateCredentialRotationPeriod(c.validityPeriod)
				regenCertTimer.Reset(rotationDuration)
				regenLogger.Debug("completed")
			case signal := <-signals:
				cred, err := c.generateCreds(logger, container, "")
				if err != nil {
					regenLogger.Error("failed-to-generate-credentials", err)
					c.metronClient.IncrementCounter(CredCreationFailedCount)
					return err
				}
				for _, h := range c.handlers {
					h.Close(cred, container)
				}
				logger.Info("signalled", lager.Data{"signal": signal.String()})
				return nil
			}
		}
	})

	return runner
}

const (
	certificatePEMBlockType = "CERTIFICATE"
	privateKeyPEMBlockType  = "RSA PRIVATE KEY"
)

func (c *credManager) generateCreds(logger lager.Logger, container executor.Container, certGUID string) (Credential, error) {
	logger = logger.Session("generating-credentials")
	logger.Info("starting")
	defer logger.Info("complete")

	logger.Debug("generating-private-key")
	privateKey, err := rsa.GenerateKey(c.entropyReader, 2048)
	if err != nil {
		return Credential{}, err
	}
	logger.Debug("generated-private-key")

	ipForCert := container.InternalIP
	if len(ipForCert) == 0 {
		ipForCert = container.ExternalIP
	}

	startValidity := c.clock.Now()

	template := createCertificateTemplate(ipForCert,
		certGUID,
		startValidity,
		startValidity.Add(c.validityPeriod),
		container.CertificateProperties.OrganizationalUnit,
	)

	logger.Debug("generating-serial-number")
	guid, err := uuid.NewV4()
	if err != nil {
		logger.Error("failed-to-generate-uuid", err)
		return Credential{}, err
	}
	logger.Debug("generated-serial-number")

	guidBytes := [16]byte(*guid)
	template.SerialNumber.SetBytes(guidBytes[:])

	logger.Debug("generating-certificate")
	certBytes, err := x509.CreateCertificate(c.entropyReader, template, c.CaCert, privateKey.Public(), c.privateKey)
	if err != nil {
		return Credential{}, err
	}
	logger.Debug("generated-certificate")

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)

	var keyBuf bytes.Buffer
	err = pemEncode(privateKeyBytes, privateKeyPEMBlockType, &keyBuf)
	if err != nil {
		return Credential{}, err
	}

	var certificateBuf bytes.Buffer
	certificateWriter := &certificateBuf
	err = pemEncode(certBytes, certificatePEMBlockType, certificateWriter)
	if err != nil {
		return Credential{}, err
	}

	err = pemEncode(c.CaCert.Raw, certificatePEMBlockType, certificateWriter)
	if err != nil {
		return Credential{}, err
	}

	creds := Credential{
		Cert: certificateBuf.String(),
		Key:  keyBuf.String(),
	}
	return creds, nil
}

func pemEncode(bytes []byte, blockType string, writer io.Writer) error {
	block := &pem.Block{
		Type:  blockType,
		Bytes: bytes,
	}
	return pem.Encode(writer, block)
}

func createCertificateTemplate(ipaddress, guid string, notBefore, notAfter time.Time, organizationalUnits []string) *x509.Certificate {
	var ipaddr []net.IP
	if len(ipaddress) == 0 {
		ipaddr = []net.IP{}
	} else {
		ipaddr = []net.IP{net.ParseIP(ipaddress)}
	}
	return &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName:         guid,
			OrganizationalUnit: organizationalUnits,
		},
		IPAddresses: ipaddr,
		DNSNames:    []string{guid},
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageKeyAgreement,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
}
