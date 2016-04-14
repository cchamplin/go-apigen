package example

// apig prefixAll:_add_
// apig packageAll:io_file
// apig standardArgs: "ctx *v8.V8Context, wg *sync.WaitGroup"
// apig stardardReturn:
// apig callbackTemplate: callback.tpl
// apig gen:os.Remove alias:unlink args:standard return:standard callback:true template:callback
// apig gen:os.Stat alias:stat args:standard return:standard callback:true template:callback
