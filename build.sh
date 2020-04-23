protoc --proto_path ../../../ -I=./proto --go_out=plugins=grpc:./proto proto/slave.proto
mv proto/github.com/brotherlogic/gobuildslave/proto/* ./proto
