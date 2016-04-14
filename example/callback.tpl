func {{.Global.Options.prefix}}{{.Definition.Alias}}({{.Definition.Arguments}}) {
	myFunc := "__internal_{{.Global.Options.package}}_{{.Definition.Alias}}{{if (.Definition.Options.callback) eq "true"}}_async{{end}}"
	ctx.AddRawFunc(myFunc, func(src v8.Loc, args ...*v8.Value) (*v8.Value, error) {
    {{if (.Definition.Options.callback) eq "true"}}
      expectedArgs := {{len .Definition.Ref.Arguments}} + 1
    {{else}}
      expectedArgs := {{len .Definition.Ref.Arguments}}
    {{end}}
    if len(args) != expectedArgs {
      return nil,fmt.Errorf("Invalid arguments")
    }
    {{range $i, $e := .Definition.Ref.Arguments}}
      arg{{$e.Name}},err := args[{{$i}}].ToString()
      if err != nil {
        return nil,fmt.Errorf("Invalid argument type")
      }
    {{end}}
    {{if (.Definition.Options.callback) eq "true"}}
      argCallback := args[expectedArgs-1]
    {{end}}

		wg.Add(1)
		go func() {
			defer wg.Done()
      {{if (len .Definition.Ref.Returns) gt 0}}
			   {{range $i, $e := .Definition.Ref.Returns}}{{if $i}}, {{end}}result{{$i}}{{end}} := {{.Definition.Method}}({{range $i, $e := .Definition.Ref.Arguments}}{{if $i}}, {{end}}arg{{$e.Name}}{{end}})
      {{else}}
         {{.Definition.Method}}({{range $i, $e := .Definition.Ref.Arguments}}{{if $i}}, {{end}}arg{{$e.Name}}{{end}})
      {{end}}
      
      
      

			f := argCallback
      
      {{range $i, $e := .Definition.Ref.Returns}}
      {{if $e.IsError}}
        var passthru{{$i}} *v8.Value
        if result{{$i}} == nil {
          passthru{{$i}}, err = ctx.ToValue("null")
        } else {
          passthru{{$i}}, err = ctx.ToValue(fmt.Sprintf("Error: %v", result{{$i}}))
        }
      {{else}}
        passthru{{$i}}, err := ctx.ToValue(result{{$i}})
      {{end}}
      
			if err != nil {
				fmt.Printf("Could not cast string %v", err)
			}
      {{end}}

			
			_, err = ctx.Apply(f, nil,  {{range $i, $e := .Definition.Ref.Returns}}{{if $i}}, {{end}}passthru{{$i}}{{end}})
			if err != nil {
				fmt.Println("Error occured calling callback %v", err)
			}
			//cb("test")
		}()

		return nil, nil
	})
}
