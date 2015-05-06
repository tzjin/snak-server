package system

import (
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"crypto/sha256"

	"github.com/go-gorp/gorp"
	"github.com/golang/glog"
	"github.com/gorilla/sessions"
	// "github.com/pelletier/go-toml"
	"github.com/robfig/cron"
	"github.com/zenazn/goji/web"
	"sniksnak/models"
)

type CsrfProtection struct {
	Key    string
	Cookie string
	Header string
	Secure bool
}

type Application struct {
	// Config         *toml.TomlTree
	Template       *template.Template
	Store          *sessions.CookieStore
	DbMap          *gorp.DbMap
	CsrfProtection *CsrfProtection
}

func (application *Application) Init() {

	// config, err := toml.LoadFile(*filename)
	// if err != nil {
	// 	glog.Fatalf("TOML load failed: %s\n", err)
	// }

	hash := sha256.New()
	io.WriteString(hash, "blankityblank")
	application.Store = sessions.NewCookieStore(hash.Sum(nil))
	application.Store.Options = &sessions.Options{
		HttpOnly: true,
		Secure:   false,
	}

	application.DbMap = models.GetDbMap()

	application.CsrfProtection = &CsrfProtection{
		Key:    "blank",
		Cookie: "blank",
		Header: "blank",
		Secure: false,
	}

	// timeformat := "01/02/2006 3:04pm MST"

	// Eastern := time.FixedZone("Eastern", -4*3600)
	// // localtime := time.Now().UTC().In(Eastern)
	// // today := localtime.Format("01-02-2006")
	// // log.Printf("%v - %v - %v\n", localtime.Format(timeformat), localtime.Unix(), localtime.Location())
	// localtime := time.Now().UTC().In(Eastern)
	// today := time.Now().Local().Add(-4 * time.Hour).Format("01-02-2006")
	// fmt.Printf("%v - %v - %v\n", localtime.Format(timeformat), localtime.UTC(), localtime.Location())
	// fmt.Println(today)

	// Setup scheduler + scraper
	c := cron.New()
	c.AddFunc("@midnight", func() { models.StoreDailyData(application.DbMap) })
	c.Start()

	if len(models.GetFoodByMeal(application.DbMap, "l")) == 0 {
		models.StoreDailyData(application.DbMap)
	}

	// application.Config = config
}

func (application *Application) LoadTemplates() error {
	var templates []string

	fn := func(path string, f os.FileInfo, err error) error {
		if f.IsDir() != true && strings.HasSuffix(f.Name(), ".html") {
			templates = append(templates, path)
		}
		return nil
	}

	err := filepath.Walk("views", fn)

	if err != nil {
		return err
	}

	application.Template = template.Must(template.ParseFiles(templates...))
	return nil
}

func (application *Application) Close() {
	glog.Info("Bye!")
}

func (application *Application) Route(controller interface{}, route string) interface{} {
	fn := func(c web.C, w http.ResponseWriter, r *http.Request) {
		c.Env["Content-Type"] = "text/html"

		methodValue := reflect.ValueOf(controller).MethodByName(route)
		methodInterface := methodValue.Interface()
		method := methodInterface.(func(c web.C, r *http.Request) (string, int))

		body, code := method(c, r)

		if session, exists := c.Env["Session"]; exists {
			err := session.(*sessions.Session).Save(r, w)
			if err != nil {
				glog.Errorf("Can't save session: %v", err)
			}
		}

		switch code {
		case http.StatusOK:
			if _, exists := c.Env["Content-Type"]; exists {
				w.Header().Set("Content-Type", c.Env["Content-Type"].(string))
			}
			io.WriteString(w, body)
		case http.StatusSeeOther, http.StatusFound:
			http.Redirect(w, r, body, code)
		}
	}
	return fn
}
