//nolint
package main

import (
	"html/template"

	"github.com/Masterminds/sprig"
)

var templatedHtml = template.Must(template.New("tinystatus").Funcs(sprig.FuncMap()).Parse(`
{{- $statuses := .Status}}
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>{{.PageTitle}}</title>
    <style>
      body { font-family: segoe ui, Roboto, Oxygen-Sans, Ubuntu, Cantarell, helvetica neue, Verdana, sans-serif; }
      h1 { margin-top: 30px; }
      ul { padding: 0px; }
      li { list-style: none; margin-bottom: 2px; padding: 5px; border-bottom: 1px solid #ddd; }
	  a { text-decoration: none; color: #000; }
      .container { max-width: 600px; width: 100%; margin: 15px auto; }
      .panel { text-align: center; padding: 10px; border: 0px; border-radius: 5px; }
      .failed-bg { color: white; background-color: #E25D6A; }
      .success-bg { color: white; background-color: #52B86A; }
      .failed { color: #E25D6A; }
      .success { color: #52B86A; }
      .small { font-size: 80%; }
      .status { float: right; }
    </style>
  </head>
  <body>
    <div class='container'>
      <h1>Global Status</h1>
      <ul>
		{{- if gt $statuses.NumberOutages 0}}
        <ul><li class='panel failed-bg'>{{$statuses.NumberOutages}} Outage(s)</li></ul>
		{{- else}}
        <li class='panel success-bg'>All Systems Operational</li>
		{{- end}}
      </ul>
	  {{- range $category, $status := $statuses.Categories}}
      <h1>{{$category}}</h1>
      <ul>
		{{- range $status}}
		  {{- if not .Succeed}}
        <li>{{.Name}} <span class='small failed'>({{.ProbeResult.Error}})</span><span class='status failed'>Disrupted</span></li>
		  {{- end}}
		{{- end}}
		{{- range $status}}
		  {{- if .Succeed}}
			{{- if (.CType | regexMatch "http[46]?") }}
        <li><a href="{{.Target}}">{{.Name}}</a> <span class='status success'>Operational</span></li>
			{{- else}}
        <li>{{.Name}} <span class='status success'>Operational</span></li>
			{{- end}}
		  {{- end}}
		{{- end}}
      </ul>
	  {{- end}}
      <p class=small> Last check: {{.LastCheck.Format "2006-01-02T15:04:05-0700"}} (in {{.Elapsed}})</p>
	  {{- with .Incidents}}
      <h1>Incidents</h1>
	    {{- range .}}
      <p>{{.}}</p>
        {{- end}}
	  {{- end}}
    </div>
  </body>
</html>
`))
