package schellyhook

import (
	_ "encoding/json"
	_ "flag"
	_ "fmt"
	_ "io/ioutil"
	"log"
	_ "net/http"
	_ "os"
	_ "regexp"
	_ "strings"

	_ "github.com/flaviostutz/schelly-webhook/schellyhook"
	_ "github.com/go-cmd/cmd"
	_ "github.com/gorilla/mux"
	_ "github.com/satori/go.uuid"
	_ "github.com/sirupsen/logrus"
)

func main() {
	log.Print("Should not start this class")
}
