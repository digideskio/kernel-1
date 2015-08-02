package handler

import (
	"fmt"
	"regexp"
	"strings"
	"strconv"
	"time"
	"os"

	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/aws"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/aws/awserr"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/service/ec2"
)

func HandleEC2Subnets(req Request) (string, map[string]interface{}, error) {
	defer recoverFailure(req)

	switch req.RequestType {
	case "Create":
		fmt.Println("CREATING SUBNETS")
		fmt.Printf("req %+v\n", req)
		return EC2SubnetsCreate(req)
	case "Update":
		fmt.Println("UPDATING SUBNETS")
		fmt.Printf("req %+v\n", req)
		return EC2SubnetsUpdate(req)
	case "Delete":
		fmt.Println("DELETING SUBNETS")
		fmt.Printf("req %+v\n", req)
		return EC2SubnetsDelete(req)
	}

	return "", nil, fmt.Errorf("unknown RequestType: %s", req.RequestType)
}

var regexMatchAvailabilityZones = regexp.MustCompile(`following availability zones: ([^.]+)`)

func EC2SubnetsCreate(req Request) (string, map[string]interface{}, error) {
	vpcId := req.ResourceProperties["Vpc"].(string)

	_, err := EC2(req).CreateSubnet(&ec2.CreateSubnetInput{
		AvailabilityZone: aws.String("garbage"),
		CIDRBlock:        aws.String("10.200.0.0/16"),
		VPCID:            aws.String(vpcId),
	})

	matches := regexMatchAvailabilityZones.FindStringSubmatch(err.Error())
	matches = strings.Split(strings.Replace(matches[1], " ", "", -1), ",")

	if len(matches) < 1 {
		return "", nil, fmt.Errorf("could not discover availability zones")
	}

	outputs := make(map[string]interface{})
	subnets := make([]string, 0, 100)
	azs := make([]string, 0, 100)

	for i, az := range matches {
		res, err := EC2(req).CreateSubnet(&ec2.CreateSubnetInput{
			AvailabilityZone: aws.String(az),
			CIDRBlock:        aws.String(fmt.Sprintf("10.0.%d.0/24", i)),
			VPCID:            aws.String(vpcId),
		})

		if err != nil {
			return "", nil, err
		}

		subnets = append(subnets, *res.Subnet.SubnetID)
		azs = append(azs, az)
		outputs["SubnetId" + strconv.Itoa(i)] = *res.Subnet.SubnetID
	}

	for i := 0; i < 10; i++ {
		// Cloudformation makes it hard to deal with a variable number of returned elements
		// This is a workaround to always send back 10 subnets, the template then lists SubnetId0-9 and cloudformation handles duplicates
		if outputs["SubnetId" + strconv.Itoa(i)] == nil {
			outputs["SubnetId" + strconv.Itoa(i)] = outputs["SubnetId0"]
		}
	}

	outputs["SubnetIds"] = subnets
	outputs["SubnetIdsString"] = strings.Join(subnets, ",")
	outputs["AvailabilityZones"] = azs
	outputs["AvailabilityZonesString"] = strings.Join(azs, ",")
	physical := strings.Join(subnets, ",")

	return physical, outputs, nil
}

func EC2SubnetsUpdate(req Request) (string, map[string]interface{}, error) {
	// nop
	return req.PhysicalResourceId, nil, nil
}

func EC2SubnetsDelete(req Request) (string, map[string]interface{}, error) {
	res, err := EC2(req).DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []*string{
					aws.String(req.ResourceProperties["Vpc"].(string)),
				},
			},
		},
	})

	if err != nil {
		return "", nil, err
	}

	for _, r := range res.Subnets {
		for {
			fmt.Fprintf(os.Stderr, "deleting: %s\n", *r.SubnetID)
			_, err := EC2(req).DeleteSubnet(&ec2.DeleteSubnetInput{
				SubnetID: aws.String(*r.SubnetID),
			})

			if err != nil {
				if awsErr, ok := err.(awserr.Error); ok {
					if awsErr.Code() == "DependencyViolation" {
						// surface error but do not return
						fmt.Fprintf(os.Stderr, "subnet error: %s\n", awsErr.Error())
					} else {
						// pass up other
						return "", nil, err
					}
				}
			} else {
				// Subnet was deleted, move on
				break
			}
			time.Sleep(2 * time.Second)
		}
	}

	return req.PhysicalResourceId, nil, nil
}
