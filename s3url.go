// +build go1.4

package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func s3UrlToParts(url string) (string, string, string) {

	if strings.HasPrefix(url,"http") {
		tparts := strings.Split(url,"?")
		parts := strings.Split(tparts[0],"/")
		bucket := strings.TrimSuffix(parts[2],".s3.amazonaws.com")
		file := parts[len(parts)-1]
		path := "/" + strings.Join(parts[3:len(parts)],"/")
		return bucket,path,file
		
	} else {
		// s3:// URL or who knows. Try.
		url = strings.TrimPrefix(url,"s3://")
		parts := strings.SplitN(url,"/",2)
		moar := strings.Split(url,"/")
		return parts[0],"/"+parts[1],moar[len(moar)-1]
	}
}

func generateS3Url(bucket string, filePath string, awsAccessKeyID string,
	awsSecretAccessKey string, httpMethod string, minuteExpire int) string {

	if httpMethod == "" {
		httpMethod = "GET"
	}

	// If you need to support multiple regions, you'll need to pass it in,
	// and mangle this var correspondingly
	endPoint := "s3.amazonaws.com"

	// In case it's not there, make sure it is
	if strings.HasPrefix(filePath,"/") == false {
		filePath = "/" + filePath
	}
	
	// String of the Epoch offset to expire the link in
	expire := fmt.Sprintf("%d", int64(time.Now().Unix())+int64(minuteExpire*60))

	// Raw string to use as the signature
	sigString := httpMethod + "\n\n\n" + expire + "\n" + "/" + bucket + filePath

	// We take the base64-encoded HMAC-SHA1 sum of the sigString,
	// using the SecretKey as the key
	mac := hmac.New(sha1.New, []byte(awsSecretAccessKey))
	mac.Write([]byte(sigString))
	msum := mac.Sum(nil)
	signature := base64.StdEncoding.EncodeToString(msum)

	// Compose the query
	query := "AWSAccessKeyId=" + url.QueryEscape(awsAccessKeyID) + "&Expires=" + expire + "&Signature=" + url.QueryEscape(signature)

	// Return the URL
	return "https://" + bucket + "." + endPoint + filePath + "?" + query
}
