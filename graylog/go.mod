module github.com/digineo/xlog/graylog

go 1.26.0

replace github.com/digineo/xlog => ../

require (
	github.com/digineo/xlog v1.1.0
	github.com/stretchr/testify v1.12.1
)

require go.yaml.in/yaml/v3 v3.0.5 // indirect
