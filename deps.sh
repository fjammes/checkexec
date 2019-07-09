export GO111MODULE=on
go mod init
go get k8s.io/client-go@release-12.0
go get k8s.io/api@kubernetes-1.14.0
go get k8s.io/apimachinery@kubernetes-1.14.0
#go get k8s.io/kubernetes/pkg/kubelet/server/remotecommand@release-1.14
