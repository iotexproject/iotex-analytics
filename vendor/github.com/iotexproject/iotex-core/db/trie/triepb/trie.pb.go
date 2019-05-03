// Code generated by protoc-gen-go. DO NOT EDIT.
// source: trie.proto

package triepb

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type BranchNodePb struct {
	Index                uint32   `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	Path                 []byte   `protobuf:"bytes,2,opt,name=path,proto3" json:"path,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BranchNodePb) Reset()         { *m = BranchNodePb{} }
func (m *BranchNodePb) String() string { return proto.CompactTextString(m) }
func (*BranchNodePb) ProtoMessage()    {}
func (*BranchNodePb) Descriptor() ([]byte, []int) {
	return fileDescriptor_4a69962149106130, []int{0}
}

func (m *BranchNodePb) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BranchNodePb.Unmarshal(m, b)
}
func (m *BranchNodePb) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BranchNodePb.Marshal(b, m, deterministic)
}
func (m *BranchNodePb) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BranchNodePb.Merge(m, src)
}
func (m *BranchNodePb) XXX_Size() int {
	return xxx_messageInfo_BranchNodePb.Size(m)
}
func (m *BranchNodePb) XXX_DiscardUnknown() {
	xxx_messageInfo_BranchNodePb.DiscardUnknown(m)
}

var xxx_messageInfo_BranchNodePb proto.InternalMessageInfo

func (m *BranchNodePb) GetIndex() uint32 {
	if m != nil {
		return m.Index
	}
	return 0
}

func (m *BranchNodePb) GetPath() []byte {
	if m != nil {
		return m.Path
	}
	return nil
}

type BranchPb struct {
	Branches             []*BranchNodePb `protobuf:"bytes,1,rep,name=branches,proto3" json:"branches,omitempty"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *BranchPb) Reset()         { *m = BranchPb{} }
func (m *BranchPb) String() string { return proto.CompactTextString(m) }
func (*BranchPb) ProtoMessage()    {}
func (*BranchPb) Descriptor() ([]byte, []int) {
	return fileDescriptor_4a69962149106130, []int{1}
}

func (m *BranchPb) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BranchPb.Unmarshal(m, b)
}
func (m *BranchPb) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BranchPb.Marshal(b, m, deterministic)
}
func (m *BranchPb) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BranchPb.Merge(m, src)
}
func (m *BranchPb) XXX_Size() int {
	return xxx_messageInfo_BranchPb.Size(m)
}
func (m *BranchPb) XXX_DiscardUnknown() {
	xxx_messageInfo_BranchPb.DiscardUnknown(m)
}

var xxx_messageInfo_BranchPb proto.InternalMessageInfo

func (m *BranchPb) GetBranches() []*BranchNodePb {
	if m != nil {
		return m.Branches
	}
	return nil
}

type LeafPb struct {
	Ext                  uint32   `protobuf:"varint,1,opt,name=ext,proto3" json:"ext,omitempty"`
	Path                 []byte   `protobuf:"bytes,2,opt,name=path,proto3" json:"path,omitempty"`
	Value                []byte   `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *LeafPb) Reset()         { *m = LeafPb{} }
func (m *LeafPb) String() string { return proto.CompactTextString(m) }
func (*LeafPb) ProtoMessage()    {}
func (*LeafPb) Descriptor() ([]byte, []int) {
	return fileDescriptor_4a69962149106130, []int{2}
}

func (m *LeafPb) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LeafPb.Unmarshal(m, b)
}
func (m *LeafPb) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LeafPb.Marshal(b, m, deterministic)
}
func (m *LeafPb) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LeafPb.Merge(m, src)
}
func (m *LeafPb) XXX_Size() int {
	return xxx_messageInfo_LeafPb.Size(m)
}
func (m *LeafPb) XXX_DiscardUnknown() {
	xxx_messageInfo_LeafPb.DiscardUnknown(m)
}

var xxx_messageInfo_LeafPb proto.InternalMessageInfo

func (m *LeafPb) GetExt() uint32 {
	if m != nil {
		return m.Ext
	}
	return 0
}

func (m *LeafPb) GetPath() []byte {
	if m != nil {
		return m.Path
	}
	return nil
}

func (m *LeafPb) GetValue() []byte {
	if m != nil {
		return m.Value
	}
	return nil
}

type ExtendPb struct {
	Path                 []byte   `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	Value                []byte   `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ExtendPb) Reset()         { *m = ExtendPb{} }
func (m *ExtendPb) String() string { return proto.CompactTextString(m) }
func (*ExtendPb) ProtoMessage()    {}
func (*ExtendPb) Descriptor() ([]byte, []int) {
	return fileDescriptor_4a69962149106130, []int{3}
}

func (m *ExtendPb) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ExtendPb.Unmarshal(m, b)
}
func (m *ExtendPb) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ExtendPb.Marshal(b, m, deterministic)
}
func (m *ExtendPb) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ExtendPb.Merge(m, src)
}
func (m *ExtendPb) XXX_Size() int {
	return xxx_messageInfo_ExtendPb.Size(m)
}
func (m *ExtendPb) XXX_DiscardUnknown() {
	xxx_messageInfo_ExtendPb.DiscardUnknown(m)
}

var xxx_messageInfo_ExtendPb proto.InternalMessageInfo

func (m *ExtendPb) GetPath() []byte {
	if m != nil {
		return m.Path
	}
	return nil
}

func (m *ExtendPb) GetValue() []byte {
	if m != nil {
		return m.Value
	}
	return nil
}

type NodePb struct {
	// Types that are valid to be assigned to Node:
	//	*NodePb_Branch
	//	*NodePb_Leaf
	//	*NodePb_Extend
	Node                 isNodePb_Node `protobuf_oneof:"node"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *NodePb) Reset()         { *m = NodePb{} }
func (m *NodePb) String() string { return proto.CompactTextString(m) }
func (*NodePb) ProtoMessage()    {}
func (*NodePb) Descriptor() ([]byte, []int) {
	return fileDescriptor_4a69962149106130, []int{4}
}

func (m *NodePb) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodePb.Unmarshal(m, b)
}
func (m *NodePb) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodePb.Marshal(b, m, deterministic)
}
func (m *NodePb) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodePb.Merge(m, src)
}
func (m *NodePb) XXX_Size() int {
	return xxx_messageInfo_NodePb.Size(m)
}
func (m *NodePb) XXX_DiscardUnknown() {
	xxx_messageInfo_NodePb.DiscardUnknown(m)
}

var xxx_messageInfo_NodePb proto.InternalMessageInfo

type isNodePb_Node interface {
	isNodePb_Node()
}

type NodePb_Branch struct {
	Branch *BranchPb `protobuf:"bytes,2,opt,name=branch,proto3,oneof"`
}

type NodePb_Leaf struct {
	Leaf *LeafPb `protobuf:"bytes,3,opt,name=leaf,proto3,oneof"`
}

type NodePb_Extend struct {
	Extend *ExtendPb `protobuf:"bytes,4,opt,name=extend,proto3,oneof"`
}

func (*NodePb_Branch) isNodePb_Node() {}

func (*NodePb_Leaf) isNodePb_Node() {}

func (*NodePb_Extend) isNodePb_Node() {}

func (m *NodePb) GetNode() isNodePb_Node {
	if m != nil {
		return m.Node
	}
	return nil
}

func (m *NodePb) GetBranch() *BranchPb {
	if x, ok := m.GetNode().(*NodePb_Branch); ok {
		return x.Branch
	}
	return nil
}

func (m *NodePb) GetLeaf() *LeafPb {
	if x, ok := m.GetNode().(*NodePb_Leaf); ok {
		return x.Leaf
	}
	return nil
}

func (m *NodePb) GetExtend() *ExtendPb {
	if x, ok := m.GetNode().(*NodePb_Extend); ok {
		return x.Extend
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*NodePb) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*NodePb_Branch)(nil),
		(*NodePb_Leaf)(nil),
		(*NodePb_Extend)(nil),
	}
}

func init() {
	proto.RegisterType((*BranchNodePb)(nil), "triepb.branchNodePb")
	proto.RegisterType((*BranchPb)(nil), "triepb.branchPb")
	proto.RegisterType((*LeafPb)(nil), "triepb.leafPb")
	proto.RegisterType((*ExtendPb)(nil), "triepb.extendPb")
	proto.RegisterType((*NodePb)(nil), "triepb.nodePb")
}

func init() { proto.RegisterFile("trie.proto", fileDescriptor_4a69962149106130) }

var fileDescriptor_4a69962149106130 = []byte{
	// 250 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x91, 0x31, 0x6b, 0xc3, 0x30,
	0x10, 0x85, 0xa3, 0xd8, 0x15, 0xe1, 0x9c, 0x96, 0x20, 0x32, 0x68, 0x34, 0xa6, 0x83, 0xe9, 0x60,
	0x8a, 0xdb, 0xa1, 0x43, 0xa7, 0xd2, 0x21, 0x53, 0x11, 0xfa, 0x07, 0xbe, 0xfa, 0x4a, 0x02, 0x41,
	0x36, 0xae, 0x5a, 0xfc, 0x2b, 0xfa, 0x9b, 0x8b, 0x74, 0x71, 0x48, 0xc0, 0xdb, 0x3b, 0xfb, 0xbd,
	0x77, 0xdf, 0x21, 0x00, 0x3f, 0x1c, 0xa8, 0xea, 0x87, 0xce, 0x77, 0x4a, 0x06, 0xdd, 0x63, 0xf1,
	0x02, 0x6b, 0x1c, 0x1a, 0xf7, 0xb9, 0xff, 0xe8, 0x5a, 0x32, 0xa8, 0xb6, 0x70, 0x73, 0x70, 0x2d,
	0x8d, 0x5a, 0xe4, 0xa2, 0xbc, 0xb5, 0x3c, 0x28, 0x05, 0x69, 0xdf, 0xf8, 0xbd, 0x5e, 0xe6, 0xa2,
	0x5c, 0xdb, 0xa8, 0x8b, 0x57, 0x58, 0x71, 0xd2, 0xa0, 0x7a, 0x9c, 0x34, 0x7d, 0x6b, 0x91, 0x27,
	0x65, 0x56, 0x6f, 0x2b, 0x5e, 0x50, 0x5d, 0xb6, 0xdb, 0xb3, 0xab, 0x78, 0x07, 0x79, 0xa4, 0xe6,
	0xcb, 0xa0, 0xda, 0x40, 0x42, 0xa3, 0x3f, 0xed, 0x0b, 0x72, 0x6e, 0x5b, 0xe0, 0xfa, 0x6d, 0x8e,
	0x3f, 0xa4, 0x93, 0xf8, 0x91, 0x87, 0xe2, 0x19, 0x56, 0x34, 0x7a, 0x72, 0xad, 0xc1, 0x73, 0x4a,
	0xcc, 0xa5, 0x96, 0x97, 0xa9, 0x3f, 0x01, 0xd2, 0xf1, 0xb9, 0x0f, 0x20, 0x19, 0x29, 0x3a, 0xb2,
	0x7a, 0x73, 0x8d, 0x6d, 0x70, 0xb7, 0xb0, 0x27, 0x87, 0xba, 0x87, 0x34, 0x20, 0x47, 0x82, 0xac,
	0xbe, 0x9b, 0x9c, 0x7c, 0xc6, 0x6e, 0x61, 0xe3, 0xdf, 0xd0, 0xc8, 0x48, 0x3a, 0xbd, 0x6e, 0x9c,
	0x40, 0x43, 0x23, 0xeb, 0x37, 0x09, 0x69, 0xe0, 0x40, 0x19, 0xdf, 0xe4, 0xe9, 0x3f, 0x00, 0x00,
	0xff, 0xff, 0x75, 0x36, 0xfd, 0x25, 0xa1, 0x01, 0x00, 0x00,
}