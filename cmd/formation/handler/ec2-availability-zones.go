package handler

import (
	"fmt"
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
	return req.PhysicalResourceId, nil, fmt.Errorf("Custom::EC2AvailabilityZones is deprecated")
}

func EC2AvailabilityZonesUpdate(req Request) (string, map[string]interface{}, error) {
	// nop
	return req.PhysicalResourceId, nil, nil
}

func EC2AvailabilityZonesDelete(req Request) (string, map[string]interface{}, error) {
	// nop
	return req.PhysicalResourceId, nil, nil
}
