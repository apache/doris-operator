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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	TlsKeyName  = "tls.key"
	TLsCertName = "tls.crt"
)

// build from secret, the secret keys should contains tls.key, tls.
func BuildCAFromSecret(s *corev1.Secret) *CA {
	if s.Data == nil {
		return nil
	}

	caBytes, ok := s.Data[TLsCertName]
	if !ok || len(caBytes) == 0 {
		klog.Infof("certificate buildCAFromSecret secret %s have not tls.crt.", s.Name)
		return nil
	}

	pkBytes, ok := s.Data[TlsKeyName]
	if !ok || len(pkBytes) == 0 {
		klog.Infof("certificate buildCAFromSecret secret %s have not tls.key.", s.Name)
		return nil
	}
	//suppose caBytes have one certificate
	cert, err := parsePemCert(caBytes)
	if err != nil {
		klog.Errorf("certificate buildCAFromSecret secret %s parse PemCert error %s.", s.Name, err.Error())
		return nil
	}
	pk, err := parsePrivateKey(pkBytes)
	if err != nil {
		klog.Errorf("certificate buildCAFromSecret secret %s parse privateKey error %s.", s.Name, err.Error())
		return nil
	}

	return &CA{
		Certificate: cert,
		cert:        pkBytes,
		PrivateKey:  pk,
		privateKey:  pkBytes,
	}
}

// parse privateKey, suppose the private key type="RSA PRIVATE KEY"
func parsePrivateKey(pkBytes []byte) (*rsa.PrivateKey, error) {
	pkb, _ := pem.Decode(pkBytes)
	pk, err := x509.ParsePKCS1PrivateKey(pkb.Bytes)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

// return the first parsed certificate.
func parsePemCert(caBytes []byte) (*x509.Certificate, error) {
	b, _ := pem.Decode(caBytes)
	if b == nil || b.Type != Cert_Type {
		return nil, errors.New("have not pem certificate")
	}

	certs, err := x509.ParseCertificates(b.Bytes)
	if err != nil {
		return nil, err
	}

	return certs[0], nil
}
