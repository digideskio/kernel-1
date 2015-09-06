package controllers

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/convox/kernel/Godeps/_workspace/src/github.com/ddollar/logger"
	docker "github.com/convox/kernel/Godeps/_workspace/src/github.com/fsouza/go-dockerclient"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/gorilla/mux"
	"github.com/convox/kernel/Godeps/_workspace/src/golang.org/x/net/websocket"

	"github.com/convox/kernel/helpers"
	"github.com/convox/kernel/models"
)

func BuildList(rw http.ResponseWriter, r *http.Request) error {
	app := mux.Vars(r)["app"]

	l := map[string]string{
		"id":      r.URL.Query().Get("id"),
		"created": r.URL.Query().Get("created"),
	}

	builds, err := models.ListBuilds(app, l)

	if err != nil {
		return err
	}

	a, err := models.GetApp(app)

	if awsError(err) == "ValidationError" {
		return RenderNotFound(rw, fmt.Sprintf("no such app: %s", app))
	}

	if err != nil {
		return err
	}

	params := map[string]interface{}{
		"App":    a,
		"Builds": builds,
	}

	if len(builds) > 0 {
		params["Last"] = builds[len(builds)-1]
	}

	return RenderJson(rw, builds)
}

func BuildGet(rw http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	app := vars["app"]
	build := vars["build"]

	_, err := models.GetApp(app)

	if awsError(err) == "ValidationError" {
		return fmt.Errorf("no such app: %s", app)
	}

	b, err := models.GetBuild(app, build)

	if err != nil && strings.HasPrefix(err.Error(), "no such build") {
		return RenderNotFound(rw, err.Error())
	}

	if err != nil {
		return err
	}

	return RenderJson(rw, b)
}

func BuildCreate(rw http.ResponseWriter, r *http.Request) error {
	build := models.NewBuild(mux.Vars(r)["app"])

	err := r.ParseMultipartForm(50 * 1024 * 1024)

	if err != nil {
		return err
	}

	err = build.Save()

	if err != nil {
		return err
	}

	resources, err := models.ListResources(os.Getenv("RACK"))

	if err != nil {
		return err
	}

	ch := make(chan error)

	source, _, err := r.FormFile("source")

	if err != nil && err != http.ErrMissingFile {
		return err
	}

	if source != nil {
		err = models.S3PutFile(resources["RegistryBucket"].Id, fmt.Sprintf("builds/%s.tgz", build.Id), source, false)

		if err != nil {
			return err
		}

		go build.ExecuteLocal(source, ch)

		err = <-ch

		if err != nil {
			return err
		} else {
			return RenderJson(rw, build)
		}
	}

	if repo := r.FormValue("repo"); repo != "" {
		go build.ExecuteRemote(repo, ch)

		err = <-ch

		if err != nil {
			return err
		} else {
			return RenderJson(rw, build)
		}
	}

	return fmt.Errorf("no source or repo")
}

func BuildLogs(ws *websocket.Conn) error {
	defer ws.Close()

	vars := mux.Vars(ws.Request())
	app := vars["app"]
	build := vars["build"]

	_, err := models.GetApp(app)

	if awsError(err) == "ValidationError" {
		return fmt.Errorf("no such app: %s", app)
	}

	_, err = models.GetBuild(app, build)

	if err != nil {
		return err
	}

	// proxy to docker container logs
	// https://docs.docker.com/reference/api/docker_remote_api_v1.19/#get-container-logs
	client, err := docker.NewClient("unix:///var/run/docker.sock")

	if err != nil {
		return err
	}

	r, w := io.Pipe()

	quit := make(chan bool)

	go scanLines(r, ws)
	go keepAlive(ws, quit)

	err = client.Logs(docker.LogsOptions{
		Container:    fmt.Sprintf("build-%s", build),
		Follow:       true,
		Stdout:       true,
		Stderr:       true,
		Tail:         "all",
		RawTerminal:  false,
		OutputStream: w,
		ErrorStream:  w,
	})

	quit <- true

	return err
}

func BuildStatus(rw http.ResponseWriter, r *http.Request) {
	log := buildsLogger("status").Start()

	vars := mux.Vars(r)
	app := vars["app"]
	id := vars["build"]

	build, err := models.GetBuild(app, id)

	if err != nil {
		helpers.Error(log, err)
		RenderError(rw, err)
		return
	}

	RenderText(rw, build.Status)
}

func buildsLogger(at string) *logger.Logger {
	return logger.New("ns=kernel cn=builds").At(at)
}

func scanLines(r io.Reader, ws *websocket.Conn) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "|", 2)

		if len(parts) < 2 {
			ws.Write([]byte(parts[0] + "\n"))
			continue
		}

		switch parts[0] {
		case "manifest":
		case "error":
			ws.Write([]byte(parts[1] + "\n"))
		default:
			ws.Write([]byte(parts[1] + "\n"))
		}
	}
}

func keepAlive(ws *websocket.Conn, quit chan bool) {
	c := time.Tick(5 * time.Second)
	b := []byte{}

	for {
		select {
		case <-c:
			ws.Write(b)
		case <-quit:
			return
		}
	}
}

func logEvent(log *logger.Logger, build models.Build, step string, err error) {
	if err != nil {
		log.Log("state=error step=build.%s app=%q build=%q error=%q", step, build.App, build.Id, err)
	} else {
		log.Success("step=build.%s app=%q build=%q", step, build.App, build.Id)
	}
}
