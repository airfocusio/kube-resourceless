// see https://stackoverflow.com/questions/37473201/is-there-a-way-to-update-the-tls-certificates-in-a-net-http-server-without-any-d
package internal

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"
)

type CertLoader struct {
	CertFile          string
	KeyFile           string
	cachedCert        *tls.Certificate
	cachedCertModTime time.Time
}

func (cr *CertLoader) GetCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	stat, err := os.Stat(cr.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed checking key file modification time: %w", err)
	}

	if cr.cachedCert == nil || stat.ModTime().After(cr.cachedCertModTime) {
		pair, err := tls.LoadX509KeyPair(cr.CertFile, cr.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed loading tls key pair: %w", err)
		}

		cr.cachedCert = &pair
		cr.cachedCertModTime = stat.ModTime()
	}

	return cr.cachedCert, nil
}
