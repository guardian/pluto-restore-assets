// Code generated by MockGen. DO NOT EDIT.
// Source: pluto-restore-assets/s3utils (interfaces: S3ClientInterface)

// Package s3utils is a generated GoMock package.
package s3utils

import (
	context "context"
	reflect "reflect"

	s3 "github.com/aws/aws-sdk-go-v2/service/s3"
	gomock "github.com/golang/mock/gomock"
)

// MockS3ClientInterface is a mock of S3ClientInterface interface.
type MockS3ClientInterface struct {
	ctrl     *gomock.Controller
	recorder *MockS3ClientInterfaceMockRecorder
}

// MockS3ClientInterfaceMockRecorder is the mock recorder for MockS3ClientInterface.
type MockS3ClientInterfaceMockRecorder struct {
	mock *MockS3ClientInterface
}

// NewMockS3ClientInterface creates a new mock instance.
func NewMockS3ClientInterface(ctrl *gomock.Controller) *MockS3ClientInterface {
	mock := &MockS3ClientInterface{ctrl: ctrl}
	mock.recorder = &MockS3ClientInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockS3ClientInterface) EXPECT() *MockS3ClientInterfaceMockRecorder {
	return m.recorder
}

// HeadObject mocks base method.
func (m *MockS3ClientInterface) HeadObject(arg0 context.Context, arg1 *s3.HeadObjectInput, arg2 ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "HeadObject", varargs...)
	ret0, _ := ret[0].(*s3.HeadObjectOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HeadObject indicates an expected call of HeadObject.
func (mr *MockS3ClientInterfaceMockRecorder) HeadObject(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HeadObject", reflect.TypeOf((*MockS3ClientInterface)(nil).HeadObject), varargs...)
}

// ListObjectsV2 mocks base method.
func (m *MockS3ClientInterface) ListObjectsV2(arg0 context.Context, arg1 *s3.ListObjectsV2Input, arg2 ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListObjectsV2", varargs...)
	ret0, _ := ret[0].(*s3.ListObjectsV2Output)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListObjectsV2 indicates an expected call of ListObjectsV2.
func (mr *MockS3ClientInterfaceMockRecorder) ListObjectsV2(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListObjectsV2", reflect.TypeOf((*MockS3ClientInterface)(nil).ListObjectsV2), varargs...)
}
