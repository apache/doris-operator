// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package tde

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	aliyunkms "github.com/alibabacloud-go/kms-20160120/v3/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	awskmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	tdev1 "github.com/apache/doris-operator/api/tde"
)

const kmsReadinessTTL = 5 * time.Minute

type kmsReadinessEntry struct {
	expiresAt time.Time
}

var (
	kmsReadinessCache sync.Map
	kmsReadinessCheck = validateKMSReadiness
)

func validateKMSReadiness(ctx context.Context, provider tdev1.ProviderType, kms *tdev1.KmsProviderStatus, accessKey, secretKey []byte) error {
	if kms == nil {
		return fmt.Errorf("KMS configuration is missing")
	}
	cacheKey := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", provider, kms.KeyID, kms.Endpoint, kms.Region, kms.Auth.CredentialSecretUID)
	if cached, ok := kmsReadinessCache.Load(cacheKey); ok && time.Now().Before(cached.(kmsReadinessEntry).expiresAt) {
		return nil
	}

	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var err error
	switch provider {
	case tdev1.ProviderAwsKms:
		err = validateAWSKMS(checkCtx, kms, accessKey, secretKey)
	case tdev1.ProviderAliyunKms:
		err = validateAliyunKMS(checkCtx, kms, accessKey, secretKey)
	default:
		return fmt.Errorf("unsupported KMS provider %s", provider)
	}
	if err != nil {
		return err
	}
	kmsReadinessCache.Store(cacheKey, kmsReadinessEntry{expiresAt: time.Now().Add(kmsReadinessTTL)})
	return nil
}

func validateAWSKMS(ctx context.Context, kms *tdev1.KmsProviderStatus, accessKey, secretKey []byte) error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(kms.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(string(accessKey), string(secretKey), "")),
	)
	if err != nil {
		return fmt.Errorf("initialize AWS KMS client: %w", err)
	}
	client := awskms.NewFromConfig(cfg, func(options *awskms.Options) {
		options.BaseEndpoint = aws.String(kms.Endpoint)
		options.RetryMaxAttempts = 2
	})
	response, err := client.DescribeKey(ctx, &awskms.DescribeKeyInput{KeyId: aws.String(kms.KeyID)})
	if err != nil {
		return fmt.Errorf("AWS KMS DescribeKey failed: %w", err)
	}
	if response.KeyMetadata == nil {
		return fmt.Errorf("AWS KMS DescribeKey returned no key metadata")
	}
	if response.KeyMetadata.KeyState != awskmstypes.KeyStateEnabled {
		return fmt.Errorf("AWS KMS key is not enabled: state=%s", response.KeyMetadata.KeyState)
	}
	probe := []byte("doris-operator-tde-readiness")
	encrypted, err := client.Encrypt(ctx, &awskms.EncryptInput{KeyId: aws.String(kms.KeyID), Plaintext: probe})
	if err != nil {
		return fmt.Errorf("AWS KMS Encrypt failed: %w", err)
	}
	if len(encrypted.CiphertextBlob) == 0 {
		return fmt.Errorf("AWS KMS Encrypt returned no ciphertext")
	}
	decrypted, err := client.Decrypt(ctx, &awskms.DecryptInput{KeyId: aws.String(kms.KeyID), CiphertextBlob: encrypted.CiphertextBlob})
	if err != nil {
		return fmt.Errorf("AWS KMS Decrypt failed: %w", err)
	}
	decryptedMatches := bytes.Equal(decrypted.Plaintext, probe)
	clear(decrypted.Plaintext)
	if !decryptedMatches {
		return fmt.Errorf("AWS KMS Decrypt returned unexpected plaintext")
	}
	dataKey, err := client.GenerateDataKey(ctx, &awskms.GenerateDataKeyInput{KeyId: aws.String(kms.KeyID), KeySpec: awskmstypes.DataKeySpecAes256})
	if err != nil {
		return fmt.Errorf("AWS KMS GenerateDataKey failed: %w", err)
	}
	if len(dataKey.Plaintext) == 0 || len(dataKey.CiphertextBlob) == 0 {
		clear(dataKey.Plaintext)
		return fmt.Errorf("AWS KMS GenerateDataKey returned incomplete key material")
	}
	clear(dataKey.Plaintext)
	return nil
}

func validateAliyunKMS(ctx context.Context, kms *tdev1.KmsProviderStatus, accessKey, secretKey []byte) error {
	endpoint, err := url.Parse(kms.Endpoint)
	if err != nil || endpoint.Host == "" {
		return fmt.Errorf("Aliyun KMS endpoint is invalid")
	}
	connectTimeout, readTimeout := 5000, 10000
	config := &openapi.Config{
		AccessKeyId:     dara.String(string(accessKey)),
		AccessKeySecret: dara.String(string(secretKey)),
		RegionId:        dara.String(kms.Region),
		Endpoint:        dara.String(endpoint.Host),
		Protocol:        dara.String("https"),
		ConnectTimeout:  &connectTimeout,
		ReadTimeout:     &readTimeout,
	}
	client, err := aliyunkms.NewClient(config)
	if err != nil {
		return fmt.Errorf("initialize Aliyun KMS client: %w", err)
	}

	resultCh := make(chan error, 1)
	go func() {
		response, callErr := client.DescribeKey(&aliyunkms.DescribeKeyRequest{KeyId: dara.String(kms.KeyID)})
		if callErr != nil {
			resultCh <- fmt.Errorf("Aliyun KMS DescribeKey failed: %w", callErr)
			return
		}
		if response == nil || response.Body == nil || response.Body.KeyMetadata == nil {
			resultCh <- fmt.Errorf("Aliyun KMS DescribeKey returned no key metadata")
			return
		}
		if dara.StringValue(response.Body.KeyMetadata.KeyState) != "Enabled" {
			resultCh <- fmt.Errorf("Aliyun KMS key is not enabled: state=%s", dara.StringValue(response.Body.KeyMetadata.KeyState))
			return
		}

		const encodedProbe = "ZG9yaXMtb3BlcmF0b3ItdGRlLXJlYWRpbmVzcw=="
		encrypted, callErr := client.Encrypt(&aliyunkms.EncryptRequest{KeyId: dara.String(kms.KeyID), Plaintext: dara.String(encodedProbe)})
		if callErr != nil {
			resultCh <- fmt.Errorf("Aliyun KMS Encrypt failed: %w", callErr)
			return
		}
		if encrypted == nil || encrypted.Body == nil || dara.StringValue(encrypted.Body.CiphertextBlob) == "" {
			resultCh <- fmt.Errorf("Aliyun KMS Encrypt returned no ciphertext")
			return
		}
		decrypted, callErr := client.Decrypt(&aliyunkms.DecryptRequest{CiphertextBlob: encrypted.Body.CiphertextBlob})
		if callErr != nil {
			resultCh <- fmt.Errorf("Aliyun KMS Decrypt failed: %w", callErr)
			return
		}
		if decrypted == nil || decrypted.Body == nil || dara.StringValue(decrypted.Body.Plaintext) != encodedProbe {
			resultCh <- fmt.Errorf("Aliyun KMS Decrypt returned unexpected plaintext")
			return
		}
		dataKey, callErr := client.GenerateDataKey(&aliyunkms.GenerateDataKeyRequest{KeyId: dara.String(kms.KeyID), KeySpec: dara.String("AES_256")})
		if callErr != nil {
			resultCh <- fmt.Errorf("Aliyun KMS GenerateDataKey failed: %w", callErr)
			return
		}
		if dataKey == nil || dataKey.Body == nil || dara.StringValue(dataKey.Body.Plaintext) == "" || dara.StringValue(dataKey.Body.CiphertextBlob) == "" {
			resultCh <- fmt.Errorf("Aliyun KMS GenerateDataKey returned incomplete key material")
			return
		}
		resultCh <- nil
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("Aliyun KMS readiness check timed out: %w", ctx.Err())
	case callErr := <-resultCh:
		return callErr
	}
}
