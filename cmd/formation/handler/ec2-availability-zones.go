package handler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/aws"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/service/ec2"
)

func HandleEC2AvailabilityZones(req Request) (string, map[string]interface{}, error) {
	defer recoverFailure(req)

	switch req.RequestType {
	case "Create":
		fmt.Println("CREATING AVAILABILITYZONES")
		fmt.Printf("req %+v\n", req)
		return EC2AvailabilityZonesCreate(req)
	case "Update":
		fmt.Println("UPDATING AVAILABILITYZONES")
		fmt.Printf("req %+v\n", req)
		return EC2AvailabilityZonesUpdate(req)
	case "Delete":
		fmt.Println("DELETING AVAILABILITYZONES")
		fmt.Printf("req %+v\n", req)
		return EC2AvailabilityZonesDelete(req)
	}

	return "", nil, fmt.Errorf("unknown RequestType: %s", req.RequestType)
}

func EC2AvailabilityZonesCreate(req Request) (string, map[string]interface{}, error) {
	_, err := EC2(req).CreateSubnet(&ec2.CreateSubnetInput{
		AvailabilityZone: aws.String("garbage"),
		CIDRBlock:        aws.String("10.200.0.0/16"),
		VPCID:            aws.String(req.ResourceProperties["Vpc"].(string)),
	})

	matches := regexMatchAvailabilityZones.FindStringSubmatch(err.Error())
	matches = strings.Split(strings.Replace(matches[1], " ", "", -1), ",")

	if len(matches) < 1 {
		return "", nil, fmt.Errorf("could not discover availability zones")
	}

	outputs := make(map[string]interface{})
	for i, az := range matches {
		outputs["AvailabilityZone"+strconv.Itoa(i)] = az
	}

	physical := strings.Join(matches, ",")
	return physical, outputs, nil
}

func EC2AvailabilityZonesUpdate(req Request) (string, map[string]interface{}, error) {
	// nop
	return req.PhysicalResourceId, nil, nil
}

func EC2AvailabilityZonesDelete(req Request) (string, map[string]interface{}, error) {
	// nop
	return req.PhysicalResourceId, nil, nil
}
