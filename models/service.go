package models

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/aws"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/service/cloudformation"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/service/dynamodb"
)

type Service struct {
	Name     string
	Password string
	Type     string
	Status   string
	URL      string

	App string

	Stack string

	Outputs    map[string]string
	Parameters map[string]string
	Tags       map[string]string
}

type Services []Service

func (s *Service) Delete() error {
	name := s.Name

	_, err := CloudFormation().DeleteStack(&cloudformation.DeleteStackInput{StackName: aws.String(name)})

	if err != nil {
		return err
	}

	return nil
}

func LinkService(app string, process string, stack string) error {
	a, err := GetApp(app)

	if err != nil {
		return err
	}

	return a.UpdateParams(map[string]string{
		UpperName(process) + "Service": stack,
	})
}

func UnlinkService(app string, stack string) error {
	a, err := GetApp(app)

	if err != nil {
		return err
	}

	p := make(map[string]string)

	for key, value := range a.Parameters {
		if strings.HasSuffix(key, "Service") && value == stack {
			p[key] = ""
			break
		}
	}

	return a.UpdateParams(p)
}

func ListServices(app string) (Services, error) {
	a, err := GetApp(app)

	if err != nil {
		return nil, err
	}

	services := make(Services, 0)

	for key, value := range a.Parameters {
		if strings.HasSuffix(key, "Service") && value != "" {
			s, err := GetServiceFromName(value)

			if err != nil {
				return nil, err
			}

			services = append(services, *s)
		}
	}

	return services, nil
}

func ListServiceStacks() (Services, error) {
	res, err := CloudFormation().DescribeStacks(&cloudformation.DescribeStacksInput{})

	if err != nil {
		return nil, err
	}

	services := make(Services, 0)

	for _, stack := range res.Stacks {
		tags := stackTags(stack)

		if tags["System"] == "convox" && tags["Type"] == "service" {
			services = append(services, *ServiceFromStack(stack))
		}
	}

	return services, nil
}

func GetService(app, name string) (*Service, error) {
	a, err := GetApp(app)

	if err != nil {
		return nil, err
	}

	value, err := s3Get(a.Outputs["Settings"], fmt.Sprintf("service/%s", name))

	if err != nil {
		return nil, err
	}

	var service *Service

	err = json.Unmarshal([]byte(value), &service)

	if err != nil {
		return nil, err
	}

	service.App = app

	return service, nil
}

func GetServiceFromName(name string) (*Service, error) {
	res, err := CloudFormation().DescribeStacks(&cloudformation.DescribeStacksInput{StackName: aws.String(name)})

	if err != nil {
		return nil, err
	}

	return ServiceFromStack(res.Stacks[0]), nil
}

func (s *Service) Create() error {
	formation, err := s.Formation()

	if err != nil {
		return err
	}

	params := map[string]string{
		"Password": s.Password,
		"Subnets":  os.Getenv("SUBNETS"),
		"Vpc":      os.Getenv("VPC"),
	}

	tags := map[string]string{
		"System":  "convox",
		"Type":    "service",
		"Service": s.Type,
	}

	req := &cloudformation.CreateStackInput{
		Capabilities: []*string{aws.String("CAPABILITY_IAM")},
		StackName:    aws.String(s.Name),
		TemplateBody: aws.String(formation),
	}

	for key, value := range params {
		req.Parameters = append(req.Parameters, &cloudformation.Parameter{ParameterKey: aws.String(key), ParameterValue: aws.String(value)})
	}

	for key, value := range tags {
		req.Tags = append(req.Tags, &cloudformation.Tag{Key: aws.String(key), Value: aws.String(value)})
	}

	_, err = CloudFormation().CreateStack(req)

	return err
}

func (s *Service) Formation() (string, error) {
	data, err := exec.Command("docker", "run", os.Getenv("DOCKER_IMAGE_SERVICE"), s.Type).Output()

	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (s *Service) Save() error {
	app, err := GetApp(s.App)

	if err != nil {
		return err
	}

	data, err := json.Marshal(s)

	if err != nil {
		return err
	}

	return S3Put(app.Outputs["Settings"], fmt.Sprintf("service/%s", s.Name), data, false)
}

func (s *Service) ManagementUrl() string {
	region := os.Getenv("AWS_REGION")

	resources, err := ListResources(s.App)

	if err != nil {
		panic(err)
	}

	switch s.Type {
	case "convox/postgres":
		id := resources[fmt.Sprintf("%sDatabase", UpperName(s.Name))].Id
		return fmt.Sprintf("https://console.aws.amazon.com/rds/home?region=%s#dbinstances:id=%s;sf=all", region, id)
	case "convox/redis":
		id := resources[fmt.Sprintf("%sInstances", UpperName(s.Name))].Id
		return fmt.Sprintf("https://console.aws.amazon.com/ec2/autoscaling/home?region=%s#AutoScalingGroups:id=%s;view=details", region, id)
	default:
		return ""
	}
}

func (s *Service) Created() bool {
	return s.Status != "creating"
}

func (s *Service) SubscribeLogs(output chan []byte, quit chan bool) error {
	done := make(chan bool)

	switch s.Tags["Service"] {
	case "postgres":
		go subscribeRDS(s.Name, s.Name, output, done)
	case "redis":
		resources, err := ListResources(s.Name)

		if err != nil {
			return err
		}

		go subscribeKinesis(resources["Kinesis"].Id, output, done)
	}
	return nil
}

func ServiceFromStack(stack *cloudformation.Stack) *Service {
	outputs := stackOutputs(stack)
	parameters := stackParameters(stack)
	tags := stackTags(stack)

	url := fmt.Sprintf("redis://u@%s:%s/%s", outputs["Port6379TcpAddr"], outputs["Port6379TcpPort"], outputs["EnvRedisDatabase"])

	if tags["Service"] == "postgres" {
		url = fmt.Sprintf("postgres://%s:%s@%s:%s/%s", outputs["EnvPostgresUsername"], outputs["EnvPostgresPassword"], outputs["Port5432TcpAddr"], outputs["Port5432TcpPort"], outputs["EnvPostgresDatabase"])
	}

	return &Service{
		Name:       cs(stack.StackName, "<unknown>"),
		Status:     humanStatus(*stack.StackStatus),
		Outputs:    outputs,
		Parameters: parameters,
		Tags:       tags,
		URL:        url,
	}
}

func servicesTable(app string) string {
	return os.Getenv("DYNAMO_SERVICES")
}

func serviceFromItem(item map[string]*dynamodb.AttributeValue) *Service {
	return &Service{
		Name: coalesce(item["name"], ""),
		Type: coalesce(item["type"], ""),
		App:  coalesce(item["app"], ""),
	}
}
