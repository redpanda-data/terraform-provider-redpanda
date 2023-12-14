### Developer Tips

#### Getting Acceptance Tests to Run

If you aren't able to get the acceptance tests to run you need to set TF_ACC=true in your environment. This is a
requirement of the framework.

Odds are you'll only read this after you've figured it out but hey, maybe I'll save someone somewhere a google.

#### NullPointerException in new Unit Tests

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

### Running Linters and Formatter

#### Prerequisites
- [golangci-lint](https://github.com/golangci/golangci-lint)
- [gofumpt](https://github.com/mvdan/gofumpt)

#### Run the commands

**Linter:**
```
$ golangci-lint run --config .golangci.yml 
```

**Formatter:**
```
$ gofumpt -w ./

# -w writes the result to source file.
```