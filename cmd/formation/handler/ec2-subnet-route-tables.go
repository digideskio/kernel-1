package handler

import (
	"fmt"
	"strings"

	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/aws"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/service/ec2"
)

func HandleEC2SubnetRouteTableAssociation(req Request) (string, map[string]interface{}, error) {
	defer recoverFailure(req)

	switch req.RequestType {
	case "Create":
		fmt.Println("CREATING SUBNET ROUTE TABLE ASSOCIATIONS")
		fmt.Printf("req %+v\n", req)
		return EC2SubnetRouteTableAssociationCreate(req)
	case "Update":
		fmt.Println("UPDATING SUBNET ROUTE TABLE ASSOCIATIONS")
		fmt.Printf("req %+v\n", req)
		return EC2SubnetRouteTableAssociationUpdate(req)
	case "Delete":
		fmt.Println("DELETING SUBNET ROUTE TABLE ASSOCIATIONS")
		fmt.Printf("req %+v\n", req)
		return EC2SubnetRouteTableAssociationDelete(req)
	}

	return "", nil, fmt.Errorf("unknown RequestType: %s", req.RequestType)
}

func EC2SubnetRouteTableAssociationCreate(req Request) (string, map[string]interface{}, error) {
	subnetIds := strings.Split(req.ResourceProperties["SubnetIds"].(string), ",")
	associations := make([]string, 0, 100)
	for _, subnetId := range subnetIds {
		res, err := EC2(req).AssociateRouteTable(&ec2.AssociateRouteTableInput{
			RouteTableID: aws.String(req.ResourceProperties["RouteTableId"].(string)),
			SubnetID:     aws.String(subnetId),
		})
		associations = append(associations, *res.AssociationID)

		if err != nil {
			return "", nil, err
		}
	}

	physical := strings.Join(associations, ",")

	return physical, nil, nil
}

func EC2SubnetRouteTableAssociationUpdate(req Request) (string, map[string]interface{}, error) {
	// nop
	return req.PhysicalResourceId, nil, nil
}

func EC2SubnetRouteTableAssociationDelete(req Request) (string, map[string]interface{}, error) {
	associationIDs := strings.Split(req.PhysicalResourceId, ",")
	for _, associaton := range associationIDs {
		_, err :=  EC2(req).DisassociateRouteTable(&ec2.DisassociateRouteTableInput{
			AssociationID: aws.String(associaton),
		})

		if err != nil {
			return "", nil, err
		}
	}

	return req.PhysicalResourceId, nil, nil
}
