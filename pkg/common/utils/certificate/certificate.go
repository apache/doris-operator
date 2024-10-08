// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"k8s.io/klog/v2"
	"math/big"
	"time"
)

var (
	// SerialNumberLimit is the maximum number used as a certificate serial number
	SerialNumberLimit    = new(big.Int).Lsh(big.NewInt(1), 128)
	DefaultExpireTimeout = 365 * 24 * time.Hour
	UpdateCABefore       = 10 * time.Minute
)

var (
	Cert_Type       = "CERTIFICATE"
	PrivateKey_Type = "RSA PRIVATE KEY"
)

type CAOptions struct {
	//Subject of location information to build.
	Subject pkix.Name
	//privateKey to be used for signing certificates(auto generated if not provided)
	PrivateKey *rsa.PrivateKey

	//all fully dns name for certificate.
	DnsNames []string
}

type CA struct {
	//PrivateKey is the unnamedwatches private key
	privateKey []byte
	//the encoded certificate.
	cert []byte
	//the struct of privateKey.
	PrivateKey *rsa.PrivateKey
	// the certificate used to issue new certificates
	Certificate *x509.Certificate
}

// create CA according to the options.
func NewCAConfigSecret(options CAOptions) (*CA, error) {
	serial, err := rand.Int(rand.Reader, SerialNumberLimit)
	if err != nil {
		return nil, errors.New("new CA failed, " + err.Error())
	}

	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.New("generate key failed, " + err.Error())
	}

	certificate := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               options.Subject,
		NotBefore:             time.Now().Add(-10 * time.Minute),
		NotAfter:              time.Now().Add(DefaultExpireTimeout),
		DNSNames:              options.DnsNames,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	cert, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &pk.PublicKey, pk)
	if err != nil {
		return nil, errors.New("failed to create certificate, " + err.Error())
	}

	return &CA{
		privateKey:  pemEncode(PrivateKey_Type, x509.MarshalPKCS1PrivateKey(pk)),
		cert:        pemEncode(Cert_Type, cert),
		PrivateKey:  pk,
		Certificate: certificate,
	}, nil
}

// return the encode signed key.
func (ca *CA) GetEncodePrivateKey() []byte {
	return ca.privateKey
}

// return the encode signed certificate.
func (ca *CA) GetEncodeCert() []byte {
	return ca.cert
}

func pemEncode(asType string, data []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: asType, Bytes: data})
}

// check the CA is still valid. privateKey convert to publickey equals to config publickey, the certificate is not expired.
func ValidCA(ca *CA) bool {
	//privatekey match publickey
	k := ca.Certificate.PublicKey.(*rsa.PublicKey)

	if !k.Equal(ca.PrivateKey.Public()) {
		return false
	}

	now := time.Now()
	if now.Before(ca.Certificate.NotBefore) {
		klog.Infof("CA cert is not valid yet, subject %s.", ca.Certificate.Subject)
		return false
	}

	if now.After(ca.Certificate.NotAfter.Add(-UpdateCABefore)) {
		return false
	}

	return true
}
