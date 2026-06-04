package node

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	panel "github.com/ZicBoard/ZicNode/api/zicboard"
	log "github.com/sirupsen/logrus"
)

const (
	domainRenewBefore = 30 * 24 * time.Hour
	ipRenewBefore     = 72 * time.Hour
)

func (c *Controller) renewCertTask(_ context.Context) error {
	if err := c.requestCert(); err != nil {
		log.WithField("tag", c.tag).Info("renew cert error: ", err)
		return nil
	}
	return nil
}

func (c *Controller) requestCert() error {
	cert := c.info.Common.CertInfo
	cert.CertMode = strings.ToLower(strings.TrimSpace(cert.CertMode))
	cert.CertDomain = normalizeCertTarget(cert.CertDomain)
	switch cert.CertMode {
	case "none", "":
	case "file":
		if err := validateCertPaths(cert); err != nil {
			return err
		}
		return ensureCertFilesExist(cert)
	case "dns", "http":
		if err := validateCertPaths(cert); err != nil {
			return err
		}
		if cert.CertDomain == "" {
			return fmt.Errorf("cert target is empty for certmode %s", cert.CertMode)
		}
		ready, err := certificateReady(cert, renewBefore(cert.CertDomain))
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		l, err := NewLego(cert)
		if err != nil {
			return fmt.Errorf("create lego object error: %s", err)
		}
		err = l.CreateCert()
		if err != nil {
			return fmt.Errorf("create lego cert error: %s", err)
		}
	case "auto":
		if err := validateCertPaths(cert); err != nil {
			return err
		}
		if err := validateAutoCertTarget(cert.CertDomain); err != nil {
			return err
		}
		ready, err := certificateReady(cert, renewBefore(cert.CertDomain))
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		l, err := NewLego(cert)
		if err != nil {
			return fmt.Errorf("create lego object error: %s", err)
		}
		err = l.CreateCert()
		if err != nil {
			return fmt.Errorf("create lego cert error: %s", err)
		}
	case "self":
		if err := validateCertPaths(cert); err != nil {
			return err
		}
		if cert.CertDomain == "" {
			return fmt.Errorf("cert target is empty for certmode self")
		}
		ready, err := certificateReady(cert, domainRenewBefore)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		err = generateSelfSslCertificate(
			cert.CertDomain,
			cert.CertFile,
			cert.KeyFile)
		if err != nil {
			return fmt.Errorf("generate self cert error: %s", err)
		}
	default:
		return fmt.Errorf("unsupported certmode: %s", cert.CertMode)
	}
	return nil
}

func validateCertPaths(cert *panel.CertInfo) error {
	if cert.CertFile == "" || cert.KeyFile == "" {
		return fmt.Errorf("cert file path or key file path not exist")
	}
	return nil
}

func ensureCertFilesExist(cert *panel.CertInfo) error {
	if _, err := os.Stat(cert.CertFile); err != nil {
		return fmt.Errorf("cert file not found %s: %w", cert.CertFile, err)
	}
	if _, err := os.Stat(cert.KeyFile); err != nil {
		return fmt.Errorf("key file not found %s: %w", cert.KeyFile, err)
	}
	return nil
}

func certificateReady(cert *panel.CertInfo, minValidity time.Duration) (bool, error) {
	if err := ensureCertFilesExist(cert); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	data, err := os.ReadFile(cert.CertFile)
	if err != nil {
		return false, fmt.Errorf("read cert file error: %s", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false, nil
	}
	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, nil
	}
	now := time.Now()
	if now.Before(x509Cert.NotBefore) || time.Until(x509Cert.NotAfter) <= minValidity {
		return false, nil
	}
	if cert.CertDomain != "" {
		if err := x509Cert.VerifyHostname(cert.CertDomain); err != nil {
			return false, nil
		}
	}
	return true, nil
}

func renewBefore(target string) time.Duration {
	if net.ParseIP(target) != nil {
		return ipRenewBefore
	}
	return domainRenewBefore
}

func validateAutoCertTarget(target string) error {
	if target == "" {
		return fmt.Errorf("auto tls requires host or tls_settings.server_name")
	}
	if ip := net.ParseIP(target); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("auto tls target %s is not a public ip", target)
		}
		return nil
	}
	if strings.EqualFold(target, "localhost") || !strings.Contains(target, ".") {
		return fmt.Errorf("auto tls target %s is not a public domain", target)
	}
	if strings.ContainsAny(target, `/\\`) {
		return fmt.Errorf("auto tls target %s is not a valid hostname", target)
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() &&
		!ip.IsPrivate() &&
		!ip.IsLoopback() &&
		!ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast()
}

func normalizeCertTarget(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Hostname() != "" {
		value = parsed.Hostname()
	} else if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.TrimPrefix(strings.TrimSuffix(value, "]"), "[")
	return strings.TrimSpace(value)
}

func generateSelfSslCertificate(domain, certPath, keyPath string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		Version:      3,
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName: domain,
		},
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(30, 0, 0),
	}
	if ip := net.ParseIP(domain); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{domain}
	}
	cert, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return err
	}
	if err := checkPath(certPath); err != nil {
		return err
	}
	f, err := os.OpenFile(certPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	err = pem.Encode(f, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})
	if err != nil {
		return err
	}
	if err := checkPath(keyPath); err != nil {
		return err
	}
	f, err = os.OpenFile(keyPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	err = pem.Encode(f, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err != nil {
		return err
	}
	return nil
}
