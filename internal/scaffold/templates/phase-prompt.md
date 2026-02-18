# Phase <ID> Implementation Prompts

## Prompt 1 of N: <Title>

### Required Reading (read these files before writing code)
- <file path>

---

### Interface Contract
```go
// <package>/<file>.go
type Xyz interface {
    Method(ctx context.Context) error
}
```

### Task

1. <Step>
2. <Step>

### Constraints
- <Constraint>

---

### Verification
```bash
go build ./...
go vet ./...
go test -race ./<package>/...
```

### Acceptance Criteria
- <Criterion>

---

## Prompt 2 of N: <Title>

### Required Reading (read these files before writing code)
- <file path>

---

### Task

1. <Step>

### Constraints
- <Constraint>

---

### Verification
```bash
go build ./...
go vet ./...
go test -race ./<package>/...
```

### Acceptance Criteria
- <Criterion>
