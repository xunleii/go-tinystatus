package main

import "html/template"

var templatedHtml = template.Must(template.New("tinystatus").Parse(`
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>tinystatus</title>
    <style>
      body { font-family: segoe ui, Roboto, Oxygen-Sans, Ubuntu, Cantarell, helvetica neue, Verdana, sans-serif; }
      h1 { margin-top: 30px; }
      ul { padding: 0px; }
      li { list-style: none; margin-bottom: 2px; padding: 5px; border-bottom: 1px solid #ddd; }
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
		{{- if gt .Status.NumberOutages 0}}
        <ul><li class='panel failed-bg'>{{.Status.NumberOutages}} Outage(s)</li></ul>
		{{- else}}
        <li class='panel success-bg'>All Systems Operational</li>
		{{- end}}
      </ul>
	  {{- range $category, $status := .Status.Categories}}
      <h1>{{$category}}</h1>
      <ul>
		{{- range $status}}
		  {{- if not .Succeed}}
        <li>{{.Name}} <span class='small failed'>({{.ProbeResult.Error}})</span><span class='status failed'>Disrupted</span></li>
		  {{- else}}
        <li>{{.Name}} <span class='status success'>Operational</span></li>
		  {{- end}}
		{{- end}}
      </ul>
	  {{- end}}
      <p class=small> Last check: {{.LastCheck.Format "2006-01-02T15:04:05-0700"}}</p>
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
