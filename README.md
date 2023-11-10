### Developer Tips

If you are getting a NPE in an otherwise correct test for a resource/data object odds are you've forgotten to set the
schema somewhere in the request or response.

```go
resp := &resource.CreateResponse{}
```

needs to be

```go
resp := &resource.CreateResponse{
    Schema: ThingSchema(),
}
```