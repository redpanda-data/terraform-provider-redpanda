package utils

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// GenerateProtobufDiffAndUpdateMask takes two proto.Message objects and calculates the
// diff in toplevel fields between them. It returns a new proto.Message object with only
// the differing values and a fieldmaskpb.FieldMask object with the names of the
// differing fields. If there are no differing fields, it returns a proto.Message object
// with no fields set and a FieldMask with an empty Paths slice.
func GenerateProtobufDiffAndUpdateMask[P interface {
	*T
	proto.Message
}, T any](newMessage, oldMessage P) (*T, *fieldmaskpb.FieldMask) {
	var t T
	diff := P(&t)
	mask := &fieldmaskpb.FieldMask{}

	fields := newMessage.ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		field := fields.Get(i)
		if !newMessage.ProtoReflect().Get(field).Equal(oldMessage.ProtoReflect().Get(field)) {
			diff.ProtoReflect().Set(field, newMessage.ProtoReflect().Get(field))
			mask.Paths = append(mask.Paths, string(field.Name()))
		}
	}
	return diff, mask
}
