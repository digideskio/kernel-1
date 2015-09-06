package helpers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

	"github.com/convox/kernel/Godeps/_workspace/src/github.com/ddollar/logger"
	"github.com/convox/kernel/Godeps/_workspace/src/github.com/stvp/rollbar"
)

func init() {
	rollbar.Token = os.Getenv("ROLLBAR_TOKEN")
}

func Error(log *logger.Logger, err error) {
	if log != nil {
		log.Error(err)
	}
	fmt.Printf("error: %s\n", err)
	rollbar.Error(rollbar.ERR, err)
}

func SendMixpanelEvent(event, message string) {
	id := os.Getenv("CLIENT_ID")
	token := os.Getenv("MIXPANEL_TOKEN")
	release := os.Getenv("RELEASE")

	m := fmt.Sprintf(`{"event": %q, "properties": {"client_id": %q, "distinct_id": %q, "message": %q, "release": %q, "token": %q}}`, event, id, id, message, release, token)
	encMessage := base64.StdEncoding.EncodeToString([]byte(m))

	_, err := http.Get(fmt.Sprintf("http://api.mixpanel.com/track/?data=%s", encMessage))

	if err != nil {
		Error(nil, err)
	}
}
