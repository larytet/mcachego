# Try go tool cover -html=coverage.out
# go tool pprof profile.out 
go test -cover  -cpuprofile profile.out -bench=. -coverprofile=coverage.out 

