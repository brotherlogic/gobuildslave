// Code generated by protoc-gen-go. DO NOT EDIT.
// source: slave.proto

/*
Package gobuildslave is a generated protocol buffer package.

It is generated from these files:
	slave.proto

It has these top-level messages:
	Empty
	Requirement
	Job
	JobAssignment
	SlaveConfig
	JobSpec
	JobDetails
	JobList
	Config
	RunRequest
	RunResponse
	UpdateRequest
	UpdateResponse
	KillRequest
	KillResponse
	ListRequest
	ListResponse
	ConfigRequest
	ConfigResponse
*/
package gobuildslave

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type RequirementCategory int32

const (
	RequirementCategory_UNKNOWN  RequirementCategory = 0
	RequirementCategory_DISK     RequirementCategory = 1
	RequirementCategory_EXTERNAL RequirementCategory = 2
)

var RequirementCategory_name = map[int32]string{
	0: "UNKNOWN",
	1: "DISK",
	2: "EXTERNAL",
}
var RequirementCategory_value = map[string]int32{
	"UNKNOWN":  0,
	"DISK":     1,
	"EXTERNAL": 2,
}

func (x RequirementCategory) String() string {
	return proto.EnumName(RequirementCategory_name, int32(x))
}
func (RequirementCategory) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type State int32

const (
	State_ACKNOWLEDGED    State = 0
	State_BUILDING        State = 1
	State_BUILT           State = 2
	State_RUNNING         State = 3
	State_UPDATE_STARTING State = 4
	State_UPDATING        State = 5
	State_KILLING         State = 6
	State_DEAD            State = 7
	State_PENDING         State = 8
	State_DIED            State = 9
)

var State_name = map[int32]string{
	0: "ACKNOWLEDGED",
	1: "BUILDING",
	2: "BUILT",
	3: "RUNNING",
	4: "UPDATE_STARTING",
	5: "UPDATING",
	6: "KILLING",
	7: "DEAD",
	8: "PENDING",
	9: "DIED",
}
var State_value = map[string]int32{
	"ACKNOWLEDGED":    0,
	"BUILDING":        1,
	"BUILT":           2,
	"RUNNING":         3,
	"UPDATE_STARTING": 4,
	"UPDATING":        5,
	"KILLING":         6,
	"DEAD":            7,
	"PENDING":         8,
	"DIED":            9,
}

func (x State) String() string {
	return proto.EnumName(State_name, int32(x))
}
func (State) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

type Empty struct {
}

func (m *Empty) Reset()                    { *m = Empty{} }
func (m *Empty) String() string            { return proto.CompactTextString(m) }
func (*Empty) ProtoMessage()               {}
func (*Empty) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type Requirement struct {
	Category   RequirementCategory `protobuf:"varint,1,opt,name=category,enum=gobuildslave.RequirementCategory" json:"category,omitempty"`
	Properties string              `protobuf:"bytes,2,opt,name=properties" json:"properties,omitempty"`
}

func (m *Requirement) Reset()                    { *m = Requirement{} }
func (m *Requirement) String() string            { return proto.CompactTextString(m) }
func (*Requirement) ProtoMessage()               {}
func (*Requirement) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *Requirement) GetCategory() RequirementCategory {
	if m != nil {
		return m.Category
	}
	return RequirementCategory_UNKNOWN
}

func (m *Requirement) GetProperties() string {
	if m != nil {
		return m.Properties
	}
	return ""
}

type Job struct {
	Name         string         `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	GoPath       string         `protobuf:"bytes,2,opt,name=go_path,json=goPath" json:"go_path,omitempty"`
	Requirements []*Requirement `protobuf:"bytes,3,rep,name=requirements" json:"requirements,omitempty"`
}

func (m *Job) Reset()                    { *m = Job{} }
func (m *Job) String() string            { return proto.CompactTextString(m) }
func (*Job) ProtoMessage()               {}
func (*Job) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *Job) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Job) GetGoPath() string {
	if m != nil {
		return m.GoPath
	}
	return ""
}

func (m *Job) GetRequirements() []*Requirement {
	if m != nil {
		return m.Requirements
	}
	return nil
}

type JobAssignment struct {
	Job    *Job   `protobuf:"bytes,1,opt,name=job" json:"job,omitempty"`
	Server string `protobuf:"bytes,2,opt,name=server" json:"server,omitempty"`
	Host   string `protobuf:"bytes,5,opt,name=host" json:"host,omitempty"`
	Port   int32  `protobuf:"varint,3,opt,name=port" json:"port,omitempty"`
	State  State  `protobuf:"varint,4,opt,name=state,enum=gobuildslave.State" json:"state,omitempty"`
}

func (m *JobAssignment) Reset()                    { *m = JobAssignment{} }
func (m *JobAssignment) String() string            { return proto.CompactTextString(m) }
func (*JobAssignment) ProtoMessage()               {}
func (*JobAssignment) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *JobAssignment) GetJob() *Job {
	if m != nil {
		return m.Job
	}
	return nil
}

func (m *JobAssignment) GetServer() string {
	if m != nil {
		return m.Server
	}
	return ""
}

func (m *JobAssignment) GetHost() string {
	if m != nil {
		return m.Host
	}
	return ""
}

func (m *JobAssignment) GetPort() int32 {
	if m != nil {
		return m.Port
	}
	return 0
}

func (m *JobAssignment) GetState() State {
	if m != nil {
		return m.State
	}
	return State_ACKNOWLEDGED
}

type SlaveConfig struct {
	Requirements []*Requirement `protobuf:"bytes,1,rep,name=requirements" json:"requirements,omitempty"`
}

func (m *SlaveConfig) Reset()                    { *m = SlaveConfig{} }
func (m *SlaveConfig) String() string            { return proto.CompactTextString(m) }
func (*SlaveConfig) ProtoMessage()               {}
func (*SlaveConfig) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *SlaveConfig) GetRequirements() []*Requirement {
	if m != nil {
		return m.Requirements
	}
	return nil
}

type JobSpec struct {
	Name     string   `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	Server   string   `protobuf:"bytes,2,opt,name=server" json:"server,omitempty"`
	Disk     int64    `protobuf:"varint,3,opt,name=disk" json:"disk,omitempty"`
	Args     []string `protobuf:"bytes,4,rep,name=args" json:"args,omitempty"`
	External bool     `protobuf:"varint,5,opt,name=external" json:"external,omitempty"`
	Host     string   `protobuf:"bytes,6,opt,name=host" json:"host,omitempty"`
	Port     int32    `protobuf:"varint,7,opt,name=port" json:"port,omitempty"`
	Cds      bool     `protobuf:"varint,8,opt,name=cds" json:"cds,omitempty"`
}

func (m *JobSpec) Reset()                    { *m = JobSpec{} }
func (m *JobSpec) String() string            { return proto.CompactTextString(m) }
func (*JobSpec) ProtoMessage()               {}
func (*JobSpec) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *JobSpec) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *JobSpec) GetServer() string {
	if m != nil {
		return m.Server
	}
	return ""
}

func (m *JobSpec) GetDisk() int64 {
	if m != nil {
		return m.Disk
	}
	return 0
}

func (m *JobSpec) GetArgs() []string {
	if m != nil {
		return m.Args
	}
	return nil
}

func (m *JobSpec) GetExternal() bool {
	if m != nil {
		return m.External
	}
	return false
}

func (m *JobSpec) GetHost() string {
	if m != nil {
		return m.Host
	}
	return ""
}

func (m *JobSpec) GetPort() int32 {
	if m != nil {
		return m.Port
	}
	return 0
}

func (m *JobSpec) GetCds() bool {
	if m != nil {
		return m.Cds
	}
	return false
}

type JobDetails struct {
	Spec      *JobSpec `protobuf:"bytes,1,opt,name=spec" json:"spec,omitempty"`
	State     State    `protobuf:"varint,2,opt,name=state,enum=gobuildslave.State" json:"state,omitempty"`
	StartTime int64    `protobuf:"varint,3,opt,name=start_time,json=startTime" json:"start_time,omitempty"`
	TestCount int32    `protobuf:"varint,4,opt,name=test_count,json=testCount" json:"test_count,omitempty"`
}

func (m *JobDetails) Reset()                    { *m = JobDetails{} }
func (m *JobDetails) String() string            { return proto.CompactTextString(m) }
func (*JobDetails) ProtoMessage()               {}
func (*JobDetails) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

func (m *JobDetails) GetSpec() *JobSpec {
	if m != nil {
		return m.Spec
	}
	return nil
}

func (m *JobDetails) GetState() State {
	if m != nil {
		return m.State
	}
	return State_ACKNOWLEDGED
}

func (m *JobDetails) GetStartTime() int64 {
	if m != nil {
		return m.StartTime
	}
	return 0
}

func (m *JobDetails) GetTestCount() int32 {
	if m != nil {
		return m.TestCount
	}
	return 0
}

type JobList struct {
	Details []*JobDetails `protobuf:"bytes,1,rep,name=details" json:"details,omitempty"`
}

func (m *JobList) Reset()                    { *m = JobList{} }
func (m *JobList) String() string            { return proto.CompactTextString(m) }
func (*JobList) ProtoMessage()               {}
func (*JobList) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{7} }

func (m *JobList) GetDetails() []*JobDetails {
	if m != nil {
		return m.Details
	}
	return nil
}

type Config struct {
	Memory      int64  `protobuf:"varint,1,opt,name=memory" json:"memory,omitempty"`
	Disk        int64  `protobuf:"varint,2,opt,name=disk" json:"disk,omitempty"`
	External    bool   `protobuf:"varint,3,opt,name=external" json:"external,omitempty"`
	GoVersion   string `protobuf:"bytes,4,opt,name=go_version,json=goVersion" json:"go_version,omitempty"`
	SupportsCds bool   `protobuf:"varint,5,opt,name=supports_cds,json=supportsCds" json:"supports_cds,omitempty"`
}

func (m *Config) Reset()                    { *m = Config{} }
func (m *Config) String() string            { return proto.CompactTextString(m) }
func (*Config) ProtoMessage()               {}
func (*Config) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{8} }

func (m *Config) GetMemory() int64 {
	if m != nil {
		return m.Memory
	}
	return 0
}

func (m *Config) GetDisk() int64 {
	if m != nil {
		return m.Disk
	}
	return 0
}

func (m *Config) GetExternal() bool {
	if m != nil {
		return m.External
	}
	return false
}

func (m *Config) GetGoVersion() string {
	if m != nil {
		return m.GoVersion
	}
	return ""
}

func (m *Config) GetSupportsCds() bool {
	if m != nil {
		return m.SupportsCds
	}
	return false
}

type RunRequest struct {
	Job *Job `protobuf:"bytes,1,opt,name=job" json:"job,omitempty"`
}

func (m *RunRequest) Reset()                    { *m = RunRequest{} }
func (m *RunRequest) String() string            { return proto.CompactTextString(m) }
func (*RunRequest) ProtoMessage()               {}
func (*RunRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{9} }

func (m *RunRequest) GetJob() *Job {
	if m != nil {
		return m.Job
	}
	return nil
}

type RunResponse struct {
}

func (m *RunResponse) Reset()                    { *m = RunResponse{} }
func (m *RunResponse) String() string            { return proto.CompactTextString(m) }
func (*RunResponse) ProtoMessage()               {}
func (*RunResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{10} }

type UpdateRequest struct {
	Job *Job `protobuf:"bytes,1,opt,name=job" json:"job,omitempty"`
}

func (m *UpdateRequest) Reset()                    { *m = UpdateRequest{} }
func (m *UpdateRequest) String() string            { return proto.CompactTextString(m) }
func (*UpdateRequest) ProtoMessage()               {}
func (*UpdateRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{11} }

func (m *UpdateRequest) GetJob() *Job {
	if m != nil {
		return m.Job
	}
	return nil
}

type UpdateResponse struct {
}

func (m *UpdateResponse) Reset()                    { *m = UpdateResponse{} }
func (m *UpdateResponse) String() string            { return proto.CompactTextString(m) }
func (*UpdateResponse) ProtoMessage()               {}
func (*UpdateResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{12} }

type KillRequest struct {
	Job *Job `protobuf:"bytes,1,opt,name=job" json:"job,omitempty"`
}

func (m *KillRequest) Reset()                    { *m = KillRequest{} }
func (m *KillRequest) String() string            { return proto.CompactTextString(m) }
func (*KillRequest) ProtoMessage()               {}
func (*KillRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{13} }

func (m *KillRequest) GetJob() *Job {
	if m != nil {
		return m.Job
	}
	return nil
}

type KillResponse struct {
}

func (m *KillResponse) Reset()                    { *m = KillResponse{} }
func (m *KillResponse) String() string            { return proto.CompactTextString(m) }
func (*KillResponse) ProtoMessage()               {}
func (*KillResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{14} }

type ListRequest struct {
}

func (m *ListRequest) Reset()                    { *m = ListRequest{} }
func (m *ListRequest) String() string            { return proto.CompactTextString(m) }
func (*ListRequest) ProtoMessage()               {}
func (*ListRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{15} }

type ListResponse struct {
	Jobs []*JobAssignment `protobuf:"bytes,1,rep,name=jobs" json:"jobs,omitempty"`
}

func (m *ListResponse) Reset()                    { *m = ListResponse{} }
func (m *ListResponse) String() string            { return proto.CompactTextString(m) }
func (*ListResponse) ProtoMessage()               {}
func (*ListResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{16} }

func (m *ListResponse) GetJobs() []*JobAssignment {
	if m != nil {
		return m.Jobs
	}
	return nil
}

type ConfigRequest struct {
}

func (m *ConfigRequest) Reset()                    { *m = ConfigRequest{} }
func (m *ConfigRequest) String() string            { return proto.CompactTextString(m) }
func (*ConfigRequest) ProtoMessage()               {}
func (*ConfigRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{17} }

type ConfigResponse struct {
	Config *SlaveConfig `protobuf:"bytes,1,opt,name=config" json:"config,omitempty"`
}

func (m *ConfigResponse) Reset()                    { *m = ConfigResponse{} }
func (m *ConfigResponse) String() string            { return proto.CompactTextString(m) }
func (*ConfigResponse) ProtoMessage()               {}
func (*ConfigResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{18} }

func (m *ConfigResponse) GetConfig() *SlaveConfig {
	if m != nil {
		return m.Config
	}
	return nil
}

func init() {
	proto.RegisterType((*Empty)(nil), "gobuildslave.Empty")
	proto.RegisterType((*Requirement)(nil), "gobuildslave.Requirement")
	proto.RegisterType((*Job)(nil), "gobuildslave.Job")
	proto.RegisterType((*JobAssignment)(nil), "gobuildslave.JobAssignment")
	proto.RegisterType((*SlaveConfig)(nil), "gobuildslave.SlaveConfig")
	proto.RegisterType((*JobSpec)(nil), "gobuildslave.JobSpec")
	proto.RegisterType((*JobDetails)(nil), "gobuildslave.JobDetails")
	proto.RegisterType((*JobList)(nil), "gobuildslave.JobList")
	proto.RegisterType((*Config)(nil), "gobuildslave.Config")
	proto.RegisterType((*RunRequest)(nil), "gobuildslave.RunRequest")
	proto.RegisterType((*RunResponse)(nil), "gobuildslave.RunResponse")
	proto.RegisterType((*UpdateRequest)(nil), "gobuildslave.UpdateRequest")
	proto.RegisterType((*UpdateResponse)(nil), "gobuildslave.UpdateResponse")
	proto.RegisterType((*KillRequest)(nil), "gobuildslave.KillRequest")
	proto.RegisterType((*KillResponse)(nil), "gobuildslave.KillResponse")
	proto.RegisterType((*ListRequest)(nil), "gobuildslave.ListRequest")
	proto.RegisterType((*ListResponse)(nil), "gobuildslave.ListResponse")
	proto.RegisterType((*ConfigRequest)(nil), "gobuildslave.ConfigRequest")
	proto.RegisterType((*ConfigResponse)(nil), "gobuildslave.ConfigResponse")
	proto.RegisterEnum("gobuildslave.RequirementCategory", RequirementCategory_name, RequirementCategory_value)
	proto.RegisterEnum("gobuildslave.State", State_name, State_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for BuildSlave service

type BuildSlaveClient interface {
	RunJob(ctx context.Context, in *RunRequest, opts ...grpc.CallOption) (*RunResponse, error)
	UpdateJob(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (*UpdateResponse, error)
	KillJob(ctx context.Context, in *KillRequest, opts ...grpc.CallOption) (*KillResponse, error)
	ListJobs(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListResponse, error)
	SlaveConfig(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (*ConfigResponse, error)
}

type buildSlaveClient struct {
	cc *grpc.ClientConn
}

func NewBuildSlaveClient(cc *grpc.ClientConn) BuildSlaveClient {
	return &buildSlaveClient{cc}
}

func (c *buildSlaveClient) RunJob(ctx context.Context, in *RunRequest, opts ...grpc.CallOption) (*RunResponse, error) {
	out := new(RunResponse)
	err := grpc.Invoke(ctx, "/gobuildslave.BuildSlave/RunJob", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *buildSlaveClient) UpdateJob(ctx context.Context, in *UpdateRequest, opts ...grpc.CallOption) (*UpdateResponse, error) {
	out := new(UpdateResponse)
	err := grpc.Invoke(ctx, "/gobuildslave.BuildSlave/UpdateJob", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *buildSlaveClient) KillJob(ctx context.Context, in *KillRequest, opts ...grpc.CallOption) (*KillResponse, error) {
	out := new(KillResponse)
	err := grpc.Invoke(ctx, "/gobuildslave.BuildSlave/KillJob", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *buildSlaveClient) ListJobs(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListResponse, error) {
	out := new(ListResponse)
	err := grpc.Invoke(ctx, "/gobuildslave.BuildSlave/ListJobs", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *buildSlaveClient) SlaveConfig(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (*ConfigResponse, error) {
	out := new(ConfigResponse)
	err := grpc.Invoke(ctx, "/gobuildslave.BuildSlave/SlaveConfig", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for BuildSlave service

type BuildSlaveServer interface {
	RunJob(context.Context, *RunRequest) (*RunResponse, error)
	UpdateJob(context.Context, *UpdateRequest) (*UpdateResponse, error)
	KillJob(context.Context, *KillRequest) (*KillResponse, error)
	ListJobs(context.Context, *ListRequest) (*ListResponse, error)
	SlaveConfig(context.Context, *ConfigRequest) (*ConfigResponse, error)
}

func RegisterBuildSlaveServer(s *grpc.Server, srv BuildSlaveServer) {
	s.RegisterService(&_BuildSlave_serviceDesc, srv)
}

func _BuildSlave_RunJob_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuildSlaveServer).RunJob(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.BuildSlave/RunJob",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuildSlaveServer).RunJob(ctx, req.(*RunRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuildSlave_UpdateJob_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuildSlaveServer).UpdateJob(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.BuildSlave/UpdateJob",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuildSlaveServer).UpdateJob(ctx, req.(*UpdateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuildSlave_KillJob_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KillRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuildSlaveServer).KillJob(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.BuildSlave/KillJob",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuildSlaveServer).KillJob(ctx, req.(*KillRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuildSlave_ListJobs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuildSlaveServer).ListJobs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.BuildSlave/ListJobs",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuildSlaveServer).ListJobs(ctx, req.(*ListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BuildSlave_SlaveConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuildSlaveServer).SlaveConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.BuildSlave/SlaveConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuildSlaveServer).SlaveConfig(ctx, req.(*ConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _BuildSlave_serviceDesc = grpc.ServiceDesc{
	ServiceName: "gobuildslave.BuildSlave",
	HandlerType: (*BuildSlaveServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RunJob",
			Handler:    _BuildSlave_RunJob_Handler,
		},
		{
			MethodName: "UpdateJob",
			Handler:    _BuildSlave_UpdateJob_Handler,
		},
		{
			MethodName: "KillJob",
			Handler:    _BuildSlave_KillJob_Handler,
		},
		{
			MethodName: "ListJobs",
			Handler:    _BuildSlave_ListJobs_Handler,
		},
		{
			MethodName: "SlaveConfig",
			Handler:    _BuildSlave_SlaveConfig_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "slave.proto",
}

// Client API for GoBuildSlave service

type GoBuildSlaveClient interface {
	Update(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*Empty, error)
	BuildJob(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*Empty, error)
	Run(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*Empty, error)
	Kill(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*Empty, error)
	List(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*JobList, error)
	GetConfig(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Config, error)
}

type goBuildSlaveClient struct {
	cc *grpc.ClientConn
}

func NewGoBuildSlaveClient(cc *grpc.ClientConn) GoBuildSlaveClient {
	return &goBuildSlaveClient{cc}
}

func (c *goBuildSlaveClient) Update(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := grpc.Invoke(ctx, "/gobuildslave.GoBuildSlave/Update", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *goBuildSlaveClient) BuildJob(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := grpc.Invoke(ctx, "/gobuildslave.GoBuildSlave/BuildJob", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *goBuildSlaveClient) Run(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := grpc.Invoke(ctx, "/gobuildslave.GoBuildSlave/Run", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *goBuildSlaveClient) Kill(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := grpc.Invoke(ctx, "/gobuildslave.GoBuildSlave/Kill", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *goBuildSlaveClient) List(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*JobList, error) {
	out := new(JobList)
	err := grpc.Invoke(ctx, "/gobuildslave.GoBuildSlave/List", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *goBuildSlaveClient) GetConfig(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Config, error) {
	out := new(Config)
	err := grpc.Invoke(ctx, "/gobuildslave.GoBuildSlave/GetConfig", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for GoBuildSlave service

type GoBuildSlaveServer interface {
	Update(context.Context, *JobSpec) (*Empty, error)
	BuildJob(context.Context, *JobSpec) (*Empty, error)
	Run(context.Context, *JobSpec) (*Empty, error)
	Kill(context.Context, *JobSpec) (*Empty, error)
	List(context.Context, *Empty) (*JobList, error)
	GetConfig(context.Context, *Empty) (*Config, error)
}

func RegisterGoBuildSlaveServer(s *grpc.Server, srv GoBuildSlaveServer) {
	s.RegisterService(&_GoBuildSlave_serviceDesc, srv)
}

func _GoBuildSlave_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JobSpec)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GoBuildSlaveServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.GoBuildSlave/Update",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GoBuildSlaveServer).Update(ctx, req.(*JobSpec))
	}
	return interceptor(ctx, in, info, handler)
}

func _GoBuildSlave_BuildJob_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JobSpec)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GoBuildSlaveServer).BuildJob(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.GoBuildSlave/BuildJob",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GoBuildSlaveServer).BuildJob(ctx, req.(*JobSpec))
	}
	return interceptor(ctx, in, info, handler)
}

func _GoBuildSlave_Run_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JobSpec)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GoBuildSlaveServer).Run(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.GoBuildSlave/Run",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GoBuildSlaveServer).Run(ctx, req.(*JobSpec))
	}
	return interceptor(ctx, in, info, handler)
}

func _GoBuildSlave_Kill_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JobSpec)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GoBuildSlaveServer).Kill(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.GoBuildSlave/Kill",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GoBuildSlaveServer).Kill(ctx, req.(*JobSpec))
	}
	return interceptor(ctx, in, info, handler)
}

func _GoBuildSlave_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GoBuildSlaveServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.GoBuildSlave/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GoBuildSlaveServer).List(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _GoBuildSlave_GetConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GoBuildSlaveServer).GetConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gobuildslave.GoBuildSlave/GetConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GoBuildSlaveServer).GetConfig(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _GoBuildSlave_serviceDesc = grpc.ServiceDesc{
	ServiceName: "gobuildslave.GoBuildSlave",
	HandlerType: (*GoBuildSlaveServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Update",
			Handler:    _GoBuildSlave_Update_Handler,
		},
		{
			MethodName: "BuildJob",
			Handler:    _GoBuildSlave_BuildJob_Handler,
		},
		{
			MethodName: "Run",
			Handler:    _GoBuildSlave_Run_Handler,
		},
		{
			MethodName: "Kill",
			Handler:    _GoBuildSlave_Kill_Handler,
		},
		{
			MethodName: "List",
			Handler:    _GoBuildSlave_List_Handler,
		},
		{
			MethodName: "GetConfig",
			Handler:    _GoBuildSlave_GetConfig_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "slave.proto",
}

func init() { proto.RegisterFile("slave.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 942 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x56, 0xef, 0x8e, 0xda, 0x46,
	0x10, 0xc7, 0x18, 0x0c, 0x1e, 0xc3, 0xc5, 0xdd, 0x6b, 0x53, 0x42, 0x9b, 0x8a, 0xb8, 0x5f, 0x2e,
	0xf9, 0x70, 0x55, 0x48, 0x54, 0x55, 0x95, 0x4e, 0x11, 0x07, 0xe8, 0x7a, 0x1c, 0xa2, 0xa7, 0x05,
	0xda, 0x7e, 0x43, 0x36, 0x6c, 0x39, 0xa7, 0xe0, 0x75, 0xbc, 0xcb, 0xa9, 0x79, 0x8b, 0xaa, 0x2f,
	0xd0, 0xbe, 0x43, 0x1f, 0xa7, 0xaf, 0xd0, 0x87, 0xa8, 0x66, 0x6d, 0x38, 0x63, 0xdc, 0x3f, 0x7c,
	0xdb, 0xfd, 0xcd, 0xcc, 0xcf, 0x33, 0xbf, 0x99, 0x1d, 0x00, 0x4b, 0xac, 0xdc, 0x7b, 0x76, 0x1e,
	0x46, 0x5c, 0x72, 0x52, 0x5b, 0x72, 0x6f, 0xe3, 0xaf, 0x16, 0x0a, 0x73, 0x2a, 0x50, 0xee, 0xaf,
	0x43, 0xf9, 0xde, 0x59, 0x81, 0x45, 0xd9, 0xbb, 0x8d, 0x1f, 0xb1, 0x35, 0x0b, 0x24, 0xb9, 0x80,
	0xea, 0xdc, 0x95, 0x6c, 0xc9, 0xa3, 0xf7, 0x0d, 0xad, 0xa5, 0x9d, 0x9d, 0xb4, 0x9f, 0x9d, 0xa7,
	0x03, 0xcf, 0x53, 0xce, 0xdd, 0xc4, 0x91, 0xee, 0x42, 0xc8, 0x67, 0x00, 0x61, 0xc4, 0x43, 0x16,
	0x49, 0x9f, 0x89, 0x46, 0xb1, 0xa5, 0x9d, 0x99, 0x34, 0x85, 0x38, 0xef, 0x40, 0x1f, 0x70, 0x8f,
	0x10, 0x28, 0x05, 0xee, 0x9a, 0xa9, 0x2f, 0x98, 0x54, 0x9d, 0xc9, 0xc7, 0x50, 0x59, 0xf2, 0x59,
	0xe8, 0xca, 0xbb, 0x24, 0xce, 0x58, 0xf2, 0x5b, 0x57, 0xde, 0x91, 0x0b, 0xa8, 0x45, 0x0f, 0x1f,
	0x15, 0x0d, 0xbd, 0xa5, 0x9f, 0x59, 0xed, 0x27, 0xff, 0x98, 0x16, 0xdd, 0x73, 0x77, 0x7e, 0xd3,
	0xa0, 0x3e, 0xe0, 0x5e, 0x47, 0x08, 0x7f, 0x19, 0xa8, 0x1a, 0x3f, 0x07, 0xfd, 0x2d, 0xf7, 0xd4,
	0xc7, 0xad, 0xf6, 0x07, 0xfb, 0x3c, 0x03, 0xee, 0x51, 0xb4, 0x92, 0xc7, 0x60, 0x08, 0x16, 0xdd,
	0xb3, 0x68, 0x9b, 0x4d, 0x7c, 0xc3, 0xd4, 0xef, 0xb8, 0x90, 0x8d, 0x72, 0x9c, 0x3a, 0x9e, 0x11,
	0x0b, 0x79, 0x24, 0x1b, 0x7a, 0x4b, 0x3b, 0x2b, 0x53, 0x75, 0x26, 0xcf, 0xa1, 0x2c, 0xa4, 0x2b,
	0x59, 0xa3, 0xa4, 0x54, 0x3c, 0xdd, 0xff, 0xcc, 0x18, 0x4d, 0x34, 0xf6, 0x70, 0x86, 0x60, 0x8d,
	0x11, 0xed, 0xf2, 0xe0, 0x47, 0x7f, 0x79, 0x50, 0xaf, 0x76, 0x5c, 0xbd, 0x7f, 0x68, 0x50, 0x19,
	0x70, 0x6f, 0x1c, 0xb2, 0x79, 0xae, 0xce, 0xff, 0x52, 0xd8, 0xc2, 0x17, 0x3f, 0xa9, 0x22, 0x74,
	0xaa, 0xce, 0x88, 0xb9, 0xd1, 0x52, 0x34, 0x4a, 0x2d, 0x1d, 0xe3, 0xf1, 0x4c, 0x9a, 0x50, 0x65,
	0x3f, 0x4b, 0x16, 0x05, 0xee, 0x4a, 0x89, 0x50, 0xa5, 0xbb, 0xfb, 0x4e, 0x1c, 0x23, 0x47, 0x9c,
	0x4a, 0x4a, 0x1c, 0x1b, 0xf4, 0xf9, 0x42, 0x34, 0xaa, 0x2a, 0x1c, 0x8f, 0xce, 0xef, 0x1a, 0xc0,
	0x80, 0x7b, 0x3d, 0x26, 0x5d, 0x7f, 0x25, 0xc8, 0x73, 0x28, 0x89, 0x90, 0xcd, 0x93, 0x1e, 0x7d,
	0x74, 0xd0, 0x23, 0xac, 0x8e, 0x2a, 0x97, 0x07, 0xa1, 0x8b, 0xff, 0x25, 0x34, 0x79, 0x0a, 0x20,
	0xa4, 0x1b, 0xc9, 0x99, 0xf4, 0xd7, 0x2c, 0x29, 0xd4, 0x54, 0xc8, 0xc4, 0x5f, 0x2b, 0xb3, 0x64,
	0x42, 0xce, 0xe6, 0x7c, 0x13, 0x48, 0xd5, 0xb7, 0x32, 0x35, 0x11, 0xe9, 0x22, 0xe0, 0x5c, 0x28,
	0x5d, 0x87, 0xbe, 0x90, 0xa4, 0x0d, 0x95, 0x45, 0x9c, 0x69, 0xd2, 0x9d, 0xc6, 0x41, 0x86, 0x49,
	0x25, 0x74, 0xeb, 0xe8, 0xfc, 0xaa, 0x81, 0x91, 0x74, 0xf8, 0x31, 0x18, 0x6b, 0xb6, 0xde, 0x3e,
	0x31, 0x9d, 0x26, 0xb7, 0x5d, 0x0b, 0x8a, 0xa9, 0x16, 0xa4, 0xe5, 0xd6, 0x33, 0x72, 0x3f, 0x05,
	0x58, 0xf2, 0xd9, 0x3d, 0x8b, 0x84, 0xcf, 0x03, 0x95, 0xb0, 0x49, 0xcd, 0x25, 0xff, 0x2e, 0x06,
	0xc8, 0x33, 0xa8, 0x89, 0x4d, 0x88, 0x82, 0x8b, 0x19, 0xca, 0x1d, 0x77, 0xcb, 0xda, 0x62, 0xdd,
	0x85, 0x70, 0x5e, 0x02, 0xd0, 0x4d, 0x80, 0xc3, 0xc4, 0xc4, 0xff, 0x7b, 0x18, 0x4e, 0x1d, 0x2c,
	0x15, 0x22, 0x42, 0x1e, 0x08, 0xe6, 0xbc, 0x86, 0xfa, 0x34, 0x5c, 0xa0, 0xc8, 0xc7, 0x90, 0xd8,
	0x70, 0xb2, 0x8d, 0x4a, 0x78, 0xda, 0x60, 0xdd, 0xf8, 0xab, 0xd5, 0x51, 0x2c, 0x27, 0x50, 0x8b,
	0x63, 0x12, 0x8e, 0x3a, 0x58, 0xd8, 0x9e, 0x84, 0xc3, 0x79, 0x03, 0xb5, 0xf8, 0x1a, 0x9b, 0xc9,
	0x17, 0x50, 0x7a, 0xcb, 0xbd, 0x6d, 0xcb, 0x3e, 0x39, 0x20, 0x7d, 0x58, 0x11, 0x54, 0x39, 0x3a,
	0x8f, 0xa0, 0x1e, 0x77, 0x6c, 0xcb, 0xd8, 0x85, 0x93, 0x2d, 0x90, 0x70, 0xbe, 0x04, 0x63, 0xae,
	0x90, 0x24, 0xd5, 0xcc, 0x33, 0x4d, 0xbd, 0x6b, 0x9a, 0x38, 0xbe, 0xf8, 0x1a, 0x4e, 0x73, 0x96,
	0x28, 0xb1, 0xa0, 0x32, 0x1d, 0xdd, 0x8c, 0xbe, 0xfd, 0x7e, 0x64, 0x17, 0x48, 0x15, 0x4a, 0xbd,
	0xeb, 0xf1, 0x8d, 0xad, 0x91, 0x1a, 0x54, 0xfb, 0x3f, 0x4c, 0xfa, 0x74, 0xd4, 0x19, 0xda, 0xc5,
	0x17, 0xbf, 0x68, 0x50, 0x56, 0x23, 0x4d, 0x6c, 0xa8, 0x75, 0xba, 0xe8, 0x3e, 0xec, 0xf7, 0xae,
	0xfa, 0x3d, 0xbb, 0x80, 0x9e, 0x97, 0xd3, 0xeb, 0x61, 0xef, 0x7a, 0x74, 0x65, 0x6b, 0xc4, 0x84,
	0x32, 0xde, 0x26, 0x76, 0x11, 0x99, 0xe9, 0x74, 0x34, 0x42, 0x5c, 0x27, 0xa7, 0xf0, 0x68, 0x7a,
	0xdb, 0xeb, 0x4c, 0xfa, 0xb3, 0xf1, 0xa4, 0x43, 0x27, 0x08, 0x96, 0x30, 0x54, 0x81, 0x78, 0x2b,
	0xa3, 0xff, 0xcd, 0xf5, 0x70, 0x88, 0x17, 0x43, 0x65, 0xd2, 0xef, 0xf4, 0xec, 0x0a, 0xc2, 0xb7,
	0xfd, 0x91, 0xa2, 0xaf, 0xc6, 0x09, 0xf6, 0x7b, 0xb6, 0xd9, 0xfe, 0xab, 0x08, 0x70, 0x89, 0x15,
	0xab, 0x5a, 0xc9, 0x1b, 0x30, 0xe8, 0x26, 0xc0, 0x25, 0x9f, 0x79, 0x13, 0x0f, 0x73, 0xd6, 0x7c,
	0x92, 0x63, 0x49, 0x5a, 0x58, 0x20, 0xdf, 0x80, 0x19, 0x8f, 0x06, 0x72, 0x64, 0x9a, 0xb4, 0x37,
	0x69, 0xcd, 0x4f, 0xf3, 0x8d, 0x3b, 0xa6, 0x4b, 0xa8, 0xe0, 0x78, 0x20, 0x4f, 0xe6, 0x8b, 0xa9,
	0x49, 0x6b, 0x36, 0xf3, 0x4c, 0x3b, 0x8e, 0x2e, 0x54, 0x71, 0x86, 0x06, 0xdc, 0x13, 0x59, 0x92,
	0xd4, 0xa8, 0x65, 0x49, 0xd2, 0x63, 0xe7, 0x14, 0xc8, 0x60, 0x7f, 0xc1, 0x67, 0x8a, 0xda, 0x1b,
	0xb1, 0x6c, 0x51, 0xfb, 0xe3, 0xe6, 0x14, 0xda, 0x7f, 0x16, 0xa1, 0x76, 0xc5, 0x53, 0x82, 0x7f,
	0x09, 0x46, 0x5c, 0x39, 0xc9, 0x5f, 0x93, 0xcd, 0xcc, 0x46, 0x8c, 0x7f, 0xf6, 0x0b, 0xe4, 0x2b,
	0xa8, 0x2a, 0x16, 0x94, 0xe7, 0xb8, 0xc8, 0x57, 0xa0, 0xd3, 0x4d, 0x70, 0x64, 0xd0, 0x6b, 0x28,
	0xa1, 0xb4, 0xc7, 0x47, 0xa9, 0x85, 0x9b, 0x67, 0x6e, 0x1e, 0x52, 0xa1, 0xaf, 0x2a, 0xcd, 0xbc,
	0x62, 0x32, 0x51, 0x3b, 0x37, 0xf4, 0xc3, 0x3c, 0x95, 0x9d, 0x82, 0x67, 0xa8, 0xff, 0x4a, 0xaf,
	0xfe, 0x0e, 0x00, 0x00, 0xff, 0xff, 0x0e, 0xcf, 0x30, 0x9a, 0x3a, 0x09, 0x00, 0x00,
}
