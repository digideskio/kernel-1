package models

import (
	"fmt"
	"math/rand"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/aws"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/stretchr/testify/assert"
	"github.com/convox/kernel/awsutil"
)

func TestNewBuild(t *testing.T) {
	rand.Seed(0)
	b := NewBuild("myapp")

	assert.Equal(t, "BCUBYHIZZKA", b.Id, "they should be equal")
	assert.Equal(t, "myapp", b.App, "they should be equal")
	assert.Equal(t, "created", b.Status, "they should be equal")
}

func TestBuildSaveError(t *testing.T) {
	b := NewBuild("myapp")
	err := b.Save()

	assert.Equal(t, "MissingRegion: could not find region configuration", err.Error())
}

func TestBuildSaveMultipleLogAttributes(t *testing.T) {
	LogMax = 16

	s := httptest.NewServer(awsutil.NewHandler([]awsutil.Cycle{
		awsutil.Cycle{
			Request: awsutil.Request{
				RequestURI: "/",
				Operation:  "DynamoDB_20120810.PutItem",
				Body:       `{"Item":{"app":{"S":"myapp"},"created":{"S":"19700101.000000.000000000"},"id":{"S":"BPMIGNCKYRW"},"logs":{"S":"hello world\ngood"},"status":{"S":"created"}},"TableName":""}`,
			},
			Response: awsutil.Response{
				StatusCode: 200,
				Body:       ``,
			},
		},
		awsutil.Cycle{
			Request: awsutil.Request{
				RequestURI: "/",
				Operation:  "DynamoDB_20120810.UpdateItem",
				Body:       `{"ExpressionAttributeValues":{":l":{"S":"bye world\n"}},"Key":{"id":{"S":"BPMIGNCKYRW"}},"TableName":"","UpdateExpression":"set logs1 = :l"}`,
			},
			Response: awsutil.Response{
				StatusCode: 200,
				Body:       ``,
			},
		},
	}))

	defer s.Close()

	aws.DefaultConfig.Region = "test"
	aws.DefaultConfig.Endpoint = s.URL

	b := NewBuild("myapp")
	b.Started = time.Unix(0, 0).UTC()
	b.Logs += fmt.Sprintf("hello world\ngoodbye world\n")
	err := b.Save()

	assert.Nil(t, err, "")
}
