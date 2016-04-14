package main

import (
	"bytes"
	"github.com/cchamplin/apigen"
)

type CallbackGenerator struct{}

func (*CallbackGenerator) OnDefine(config apigen.GlobalConfig, def apigen.DefConfig) (string, error) {
	var buffer bytes.Buffer
	buffer.WriteString("myFunc := __internal_")
	buffer.WriteString(config.Options["package"])
	buffer.WriteString("_")
	buffer.WriteString(def.Alias)

	if def.Options["callback"] == "true" {
		buffer.WriteString("_async")
	}
	return "", nil
}

func main() {
	apigen.Process()
}
