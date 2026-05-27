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
//
// Use this when the server tolerates a sparse Update payload — cluster and
// shadowlink rely on it today. For APIs whose `buf.validate.field` rules run
// on the full message regardless of FieldMask (and would reject zero-valued
// unchanged required fields), use PlanPayloadWithUpdateMask instead.
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
		newHas := newMsg.ProtoReflect().Has(field)
		oldHas := oldMsg.ProtoReflect().Has(field)
		if newHas != oldHas {
			// Presence change: one side has the field set, other doesn't.
			// For singular message fields, Get() returns the same zero-value
			// on both sides even when one pointer is nil — Has() catches this.
			if newHas {
				diffMsg.ProtoReflect().Set(field, newMsg.ProtoReflect().Get(field))
			} else {
				diffMsg.ProtoReflect().Clear(field)
			}
			mask.Paths = append(mask.Paths, string(field.Name()))
		} else if newHas && !newMsg.ProtoReflect().Get(field).Equal(oldMsg.ProtoReflect().Get(field)) {
			// Both present, values differ.
			diffMsg.ProtoReflect().Set(field, newMsg.ProtoReflect().Get(field))
			mask.Paths = append(mask.Paths, string(field.Name()))
		}
	}
	return diff, mask
}

// PlanPayloadWithUpdateMask returns the plan (new) message unchanged plus a
// FieldMask listing the field paths that differ from the state (old) message.
// Use this — instead of GenerateProtobufDiffAndUpdateMask — when the server's
// `buf.validate.field` annotations run on the full message regardless of
// FieldMask, and zero-valued unchanged fields would trip required-field
// validators (string.min_len, regex, etc.).
//
// redpanda_service_account is the canonical consumer: its `name` field has
// `string.min_len = 3` + regex rules that fail on the empty value the sparse
// helper would produce when name is unchanged.
//
// GenerateProtobufDiffAndUpdateMask is kept as-is — cluster and shadowlink
// have been manually verified against the sparse contract and don't need a
// different shape.
func PlanPayloadWithUpdateMask[T any, P interface {
	*T
	protoreflect.ProtoMessage
}](newMessage, oldMessage P) (P, *fieldmaskpb.FieldMask) {
	_, mask := GenerateProtobufDiffAndUpdateMask(newMessage, oldMessage)
	return newMessage, mask
}
