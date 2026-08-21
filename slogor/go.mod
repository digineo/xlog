module github.com/digineo/xlog/slogor

go 1.26.0

replace github.com/digineo/xlog => ../

require (
	github.com/digineo/xlog v1.1.0
	github.com/mattn/go-isatty v0.0.24
	github.com/stretchr/testify v1.12.1
	gitlab.com/greyxor/slogor v1.7.0
)

require (
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
