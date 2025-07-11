package utils

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// GenerateProtobufDiffAndUpdateMask takes two protoreflect.ProtoMessage objects and calculates the
// diff in toplevel fields between them. It returns a new ProtoMessage object with only
// the differing values and a fieldmaskpb.FieldMask object with the names of the
// differing fields. If there are no differing fields, it returns a ProtoMessage object
// with no fields set and a FieldMask with an empty Paths slice.
func GenerateProtobufDiffAndUpdateMask[T any, P interface {
	*T
	protoreflect.ProtoMessage
}](newMessage, oldMessage P) (*T, *fieldmaskpb.FieldMask) {
	var t T
	diff := P(&t)
	mask := &fieldmaskpb.FieldMask{}

	// Cast to protoreflect.ProtoMessage to access methods
	newMsg := protoreflect.ProtoMessage(newMessage)
	oldMsg := protoreflect.ProtoMessage(oldMessage)
	diffMsg := protoreflect.ProtoMessage(diff)

	fields := newMsg.ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		field := fields.Get(i)
		if !newMsg.ProtoReflect().Get(field).Equal(oldMsg.ProtoReflect().Get(field)) {
			diffMsg.ProtoReflect().Set(field, newMsg.ProtoReflect().Get(field))
			mask.Paths = append(mask.Paths, string(field.Name()))
		}
	}
	return diff, mask
}
