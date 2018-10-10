# Try go tool cover -html=coverage-cache.out
# go tool pprof profile-cache.out 

wd=`dirname $0`
go test -cover  -cpuprofile profile-cache.out -bench=. -coverprofile=coverage-cache.out $wd 
go test -parallel=1  -cover  -cpuprofile profile-hashtable.out -bench=. -coverprofile=coverage-hashtable.out $wd/hashtable
go test -cover  -cpuprofile profile-unsafepool.out -bench=. -coverprofile=coverage-unsafepool.out $wd/unsafepool

