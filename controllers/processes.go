package controllers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/convox/kernel/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/aws/awserr"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/gorilla/mux"
	"github.com/convox/kernel/Godeps/_workspace/src/golang.org/x/net/websocket"

	"github.com/convox/kernel/models"
)

func ProcessList(rw http.ResponseWriter, r *http.Request) error {
	app := mux.Vars(r)["app"]

	_, err := models.GetApp(app)

	if awsError(err) == "ValidationError" {
		return RenderNotFound(rw, fmt.Sprintf("no such app: %s", app))
	}

	processes, err := models.ListProcesses(app)

	if err != nil {
		return err
	}

	return RenderJson(rw, processes)
}

func ProcessShow(rw http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	app := vars["app"]
	process := vars["process"]

	_, err := models.GetApp(app)

	if awsError(err) == "ValidationError" {
		return RenderNotFound(rw, fmt.Sprintf("no such app: %s", app))
	}

	p, err := models.GetProcess(app, process)

	if err != nil {
		return err
	}

	return RenderJson(rw, p)
}

func ProcessScale(rw http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	app := vars["app"]
	process := vars["process"]

	a, err := models.GetApp(app)

	if err != nil {
		return err
	}

	p, err := models.GetProcess(app, process)

	if err != nil {
		return err
	}

	params := map[string]string{}

	un := models.UpperName(process)

	if count := GetForm(r, "count"); count != "" {
		params[un+"DesiredCount"] = count
	}

	if mem := GetForm(r, "memory"); mem != "" {
		params[un+"Memory"] = mem
	}

	if len(params) > 0 {
		err := a.UpdateParams(params)

		if ae, ok := err.(awserr.Error); ok {
			if ae.Code() == "ValidationError" {
				switch {
				case strings.Index(ae.Error(), "No updates are to be performed") > -1:
					return fmt.Errorf("no updates are to be performed: %s", app)
				case strings.Index(ae.Error(), "can not be updated") > -1:
					return fmt.Errorf("app is already updating: %s", app)
				}
			}
		}

		if err != nil {
			return err
		}
	}

	return RenderJson(rw, p)
}

func ProcessRunDetached(rw http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	app := vars["app"]
	process := vars["process"]
	command := GetForm(r, "command")

	a, err := models.GetApp(app)

	if awsError(err) == "ValidationError" {
		return RenderNotFound(rw, fmt.Sprintf("no such app: %s", app))
	}

	err = a.RunDetached(process, command)

	if err != nil {
		return err
	}

	return RenderSuccess(rw)
}

func ProcessRunAttached(ws *websocket.Conn) error {
	defer ws.Close()

	vars := mux.Vars(ws.Request())
	app := vars["app"]
	process := vars["process"]
	command := ws.Request().Header.Get("Command")

	a, err := models.GetApp(app)

	if awsError(err) == "ValidationError" {
		return fmt.Errorf("no such app: %s", app)
	}

	if err != nil {
		return err
	}

	return a.RunAttached(process, command, ws)
}

func ProcessStop(rw http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	app := vars["app"]
	process := vars["process"]

	_, err := models.GetApp(app)

	if awsError(err) == "ValidationError" {
		return RenderNotFound(rw, fmt.Sprintf("no such app: %s", app))
	}

	ps, err := models.GetProcess(app, process)

	if err != nil {
		return err
	}

	if ps == nil {
		return RenderNotFound(rw, fmt.Sprintf("no such process: %s", process))
	}

	err = ps.Stop()

	if err != nil {
		return err
	}

	return RenderJson(rw, ps)
}

// func ProcessTop(rw http.ResponseWriter, r *http.Request) {
//   log := processesLogger("info").Start()

//   vars := mux.Vars(r)
//   app := vars["app"]
//   id := vars["id"]

//   _, err := models.GetApp(app)

//   if awsError(err) == "ValidationError" {
//     RenderNotFound(rw, fmt.Sprintf("no such app: %s", app))
//     return
//   }

//   ps, err := models.GetProcessById(app, id)

//   if err != nil {
//     helpers.Error(log, err)
//     RenderError(rw, err)
//     return
//   }

//   if ps == nil {
//     RenderNotFound(rw, fmt.Sprintf("no such process: %s", id))
//     return
//   }

//   info, err := ps.Top()

//   if err != nil {
//     helpers.Error(log, err)
//     RenderError(rw, err)
//     return
//   }

//   RenderJson(rw, info)
// }

// func ProcessTypeTop(rw http.ResponseWriter, r *http.Request) {
//   log := processesLogger("info").Start()

//   vars := mux.Vars(r)
//   app := vars["app"]
//   process := vars["process_type"]

//   _, err := models.GetApp(app)

//   if awsError(err) == "ValidationError" {
//     RenderNotFound(rw, fmt.Sprintf("no such app: %s", app))
//     return
//   }

//   params := &cloudwatch.ListMetricsInput{
//     Namespace: aws.String("AWS/ECS"),
//   }

//   output, err := models.CloudWatch().ListMetrics(params)

//   if err != nil {
//     helpers.Error(log, err)
//     RenderError(rw, err)
//     return
//   }

//   var outputs []*cloudwatch.GetMetricStatisticsOutput
//   serviceStr := fmt.Sprintf("%s-%s", app, process)

//   for _, metric := range output.Metrics {
//     for _, dimension := range metric.Dimensions {
//       if (*dimension.Name == "ServiceName") && (strings.Contains(*dimension.Value, serviceStr)) {

//         params := &cloudwatch.GetMetricStatisticsInput{
//           MetricName: aws.String(*metric.MetricName),
//           StartTime:  aws.Time(time.Now().Add(-2 * time.Minute)),
//           EndTime:    aws.Time(time.Now()),
//           Period:     aws.Long(60),
//           Namespace:  aws.String("AWS/ECS"),
//           Statistics: []*string{
//             aws.String("Maximum"),
//             aws.String("Average"),
//             aws.String("Minimum"),
//           },
//           Dimensions: metric.Dimensions,
//         }

//         output, err := models.CloudWatch().GetMetricStatistics(params)

//         if err != nil {
//           RenderError(rw, err)
//           return
//         }

//         if output.Datapoints != nil {
//           outputs = append(outputs, output)
//         }
//       }
//     }
//   }

//   RenderJson(rw, outputs)
// }

func copyWait(w io.Writer, r io.Reader, wg *sync.WaitGroup) {
	io.Copy(w, r)
	wg.Done()
}
