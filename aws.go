package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func initAWS(awsRegion, awsAccessKey, awsSecretKey string) (AWSSession *session.Session) {

	AWSSession = session.New()

	// Region
	if awsRegion != "" {
		// CLI trumps
		AWSSession.Config.Region = aws.String(awsRegion)
	} else {
		region, err := getAwsRegionE()

		if err != nil {
			log.Fatalf("Cannot set AWS region: '%v'\n", err)
		}
		AWSSession.Config.Region = aws.String(region)
	}

	// Creds
	if awsAccessKey != "" && awsSecretKey != "" {
		// CLI trumps
		creds := credentials.NewStaticCredentials(
			awsAccessKey,
			awsSecretKey,
			"")
		AWSSession.Config.Credentials = creds
	}

	return

}

func getAwsRegion() (region string) {
	region, _ = getAwsRegionE()
	return
}

func getAwsRegionE() (region string, err error) {

	if os.Getenv("AWS_REGION") != "" {
		region = os.Getenv("AWS_REGION")
	} else {
		// Grab it from this EC2 instace
		region, err = ec2metadata.New(session.New()).Region()
	}
	return
}

func getEc2Instances(AWSSession *session.Session) (diout *ec2.DescribeInstancesOutput, err error) {

	svc := ec2.New(AWSSession)

	params := &ec2.DescribeInstancesInput{
		/*InstanceIds: []*string{
			aws.String("i-03b4b684"),
		},*/
	}
	diout, err = svc.DescribeInstances(params)

	return
}

func newHostFromInstance(inst *ec2.Instance) (h Host) {

	//fmt.Println(inst)

	h = Host{
		Address: *inst.PrivateIpAddress,
		Arch:    *inst.Architecture,
		Loc:     *inst.Placement.AvailabilityZone,
	}

	if *inst.State.Name != "running" {
		h.Offline = true
	}

	var tags []string
	for _, t := range inst.Tags {
		// Handle special Tags
		if *t.Key == "Name" {
			h.Name = *t.Value
		} else if *t.Key == "sshuser" {
			h.User = *t.Value
		} else if *t.Key == "sshport" {
			h.Port, _ = strconv.Atoi(*t.Value)
		} else if *t.Key == "wave" {
			h.Wave, _ = strconv.Atoi(*t.Value)
		} else if *t.Key == "noall" {
			// They don't want our help
			h.Offline = true
		} else if *t.Key == "dontupdatepackages" {
			// They don't want certain yum updates
			h.DontUpdatePackages = *t.Value
		} else {
			var tag string
			if t.Value == nil || *t.Value == "" {
				tag = *t.Key
			} else {
				tag = fmt.Sprintf("%s|%s", *t.Key, *t.Value)
			}
			tags = append(tags, tag)
		}
	}
	h.Tags = tags

	return
}
