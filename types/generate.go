//go:generate GOWORK=off go run ../internal/cmd/fixgen -xsd ../aixm-5.1.1/AIXM_DataTypes.xsd -out datatypes.go -pkg types

package types
