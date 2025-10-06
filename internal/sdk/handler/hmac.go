// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package handler

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
)

// HeaderTaskSignature is the HTTP header Terraform sets with the hex-encoded HMAC of the request body.
const HeaderTaskSignature = "X-Tfc-Task-Signature"

func VerifyHMAC(requestBody []byte, requestSignature []byte, key []byte) (bool, error) {
	mac := hmac.New(sha512.New, key)
	_, err := mac.Write(requestBody)

	if err != nil {
		return false, err
	}

	// Request signatures are hexadecimal encoded.
	return hmac.Equal(requestSignature, []byte(hex.EncodeToString(mac.Sum(nil)))), nil
}
