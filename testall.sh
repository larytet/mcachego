# Try go tool cover -html=coverage.out
# go tool pprof profile.out 

wd=`dirname $0`
go test -cover  -cpuprofile profile.out -bench=. -coverprofile=coverage.out $wd/cache 

