package render

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/CloudyKit/jet/v6"
	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
)

// struct for renderer
type Render struct {
	Renderer   string
	RootPath   string
	Secure     bool
	Port       string
	ServerName string
	JetViews   *jet.Set
	Session    *scs.SessionManager
}

type TemplateData struct {
	IsAuthenticated bool
	IntMap          map[string]int
	StringMap       map[string]string
	FloatMap        map[string]float32
	Data            map[string]any
	CSRFToken       string
	Port            string
	ServerName      string
	Secure          bool
	Error           string
	Flash           string
}

func (c *Render) defaultData(td *TemplateData, r *http.Request) *TemplateData {
	td.Secure = c.Secure
	td.ServerName = c.ServerName
	td.CSRFToken = nosurf.Token(r)
	td.Port = c.Port
	if c.Session.Exists(r.Context(), "userID") {
		td.IsAuthenticated = true
	}
	td.Error = c.Session.PopString(r.Context(), "error")
	td.Flash = c.Session.PopString(r.Context(), "flash")
	return td
}

func (rdr *Render) Page(w http.ResponseWriter, r *http.Request, view string, variables, data any) error {
	switch strings.ToLower(rdr.Renderer) {
	case "go":
		return rdr.GoPage(w, r, view, data)
	case "jet":
		return rdr.JetPage(w, r, view, variables, data)
	}
	return nil
}

// renders page with go template
func (rdr *Render) GoPage(w http.ResponseWriter, r *http.Request, view string, data any) error {
	template, err := template.ParseFiles(fmt.Sprintf("%s/views/%s.page.tmpl", rdr.RootPath, view))
	if err != nil {
		return err
	}

	templateData := &TemplateData{}

	if data != nil {
		templateData = data.(*TemplateData)
	}

	template.Execute(w, templateData)

	return nil
}

// renders page with jet enjine
func (rdr *Render) JetPage(w http.ResponseWriter, r *http.Request, templateName string, variables, data any) error {
	var vars jet.VarMap

	if variables == nil {
		vars = make(jet.VarMap)
	} else {
		vars = variables.(jet.VarMap)
	}

	template := &TemplateData{}
	if data != nil {
		template = data.(*TemplateData)
	}

	template = rdr.defaultData(template, r)

	tem, err := rdr.JetViews.GetTemplate(fmt.Sprintf("%s.jet", templateName))
	if err != nil {
		log.Println(err)
		return err
	}

	if err = tem.Execute(w, vars, template); err != nil {
		log.Println(err)
		return err
	}

	return nil
}
