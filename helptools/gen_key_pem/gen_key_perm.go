package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"gitee.com/dark.H/gs"
)

func main() {
	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		panic(err)
	}
	w := gs.Str(".").PathJoin("..", "..", "Resources", "pem")
	if !w.IsExists() {
		w = gs.Str(".").PathJoin("Resources", "pem")
		if !w.IsExists() {
			gs.Str("must dir is exsits in " + w).Println()
			return
		}
	}
	// Create a self-signed certificate
	// ca := &x509.Certificate{
	// 	SerialNumber: big.NewInt(2019),
	// 	Subject: pkix.Name{
	// 		Organization:  []string{"Company, INC."},
	// 		Country:       []string{"US"},
	// 		Province:      []string{""},
	// 		Locality:      []string{"San Francisco"},
	// 		StreetAddress: []string{"Golden Gate Bridge"},
	// 		PostalCode:    []string{"94016"},
	// 	},

	// 	NotBefore:             time.Now(),
	// 	NotAfter:              time.Now().AddDate(100, 0, 0),
	// 	SubjectKeyId:          []byte{1, 2, 3, 4, 6},
	// 	ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	// 	KeyUsage:              x509.KeyUsageDigitalSignature,
	// 	BasicConstraintsValid: true,
	// }

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2022),
		Subject: pkix.Name{
			Organization:  []string{gs.Str("").RandStr(5).Str() + "Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(100, 0, 0),

		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	cert.IsCA = true
	host := "localhost,127.0.0.1:55443"
	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			cert.IPAddresses = append(cert.IPAddresses, ip)
		} else {
			cert.DNSNames = append(cert.DNSNames, h)
		}
	}
	certDER, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
	if err != nil {
		panic(err)
	}

	// PEM encode the private key and certificate
	keyOut, err := os.OpenFile(gs.Str(w).PathJoin("key.pem").Str(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	keyOut.Close()

	certOut, err := os.OpenFile(gs.Str(w).PathJoin("cert.pem").Str(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()
}
