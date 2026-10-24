# Konsep yang dipakai Firman Message Board

Dokumen ini menjelaskan istilah yang muncul di [README](../README.md). Ikuti langkah perintah pada README; halaman ini menjelaskan hubungan antarkomponen.

## Docker, image, dan Amazon ECR

**Dockerfile** adalah resep untuk membangun image aplikasi Go. Image berisi program serta filesystem yang dibutuhkan container. **Amazon ECR** adalah registry AWS tempat image disimpan. Setiap image rilis memakai tag **commit SHA** sehingga versi yang berjalan dapat ditelusuri ke commit Git tertentu. Semua tiga cluster memakai **satu repository ECR private** di Jakarta.

GitHub Actions masuk ke AWS memakai **OIDC**: workflow meminta identitas sementara dari AWS IAM role, lalu push image ke ECR. Tidak ada AWS access key jangka panjang di GitHub. Terraform registry membuat ECR dan role tersebut. Role push hanya untuk repository ECR demo ini dan branch `main`.

ECR private perlu autentikasi saat **pull**. EKS memakai IAM role pada node. Kubernetes manual EC2 dan OpenShift memakai Kubernetes Secret `ecr-pull`, yang berisi token ECR dari identitas AWS operator. Token berlaku **12 jam**; jalankan `scripts/refresh-ecr-pull-secret.py` sebelum token habis atau sebelum rollout berikutnya. Secret tidak disimpan di Git. [Dokumentasi autentikasi ECR](https://docs.aws.amazon.com/AmazonECR/latest/userguide/registry_auth.html).

## Kubernetes, kubeadm, dan node

**Kubernetes** menjaga agar container yang dideklarasikan tetap berjalan. `Deployment` menyatakan image, jumlah Pod, batas resource, dan health probe; `Service` memberi alamat stabil di dalam cluster. `Namespace` memisahkan objek aplikasi. **Control plane** menjalankan API dan komponen pengendali cluster; **worker** menjalankan Pod aplikasi. Pada use case EC2 ada satu EC2 untuk tiap peran. **kubeadm** menyiapkan control plane dan menggabungkan worker; **containerd** menjalankan container, sementara **Flannel** memberi jaringan antar-Pod. Perintah `kubectl apply -k k8s/overlays/ec2` mengirim manifest aplikasi setelah kedua node `Ready`. Karena masing-masing EC2 hanya 2 GiB RAM, aplikasi dirilis manual tanpa Argo CD. [Panduan kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/).

**EKS** adalah Kubernetes dengan control plane yang dikelola AWS. Terraform membuat control plane dan satu **managed node group** berisi dua EC2 worker node. Pada Kubernetes EC2 manual, kita menyebut mesin aplikasi sebagai worker, karena Terraform membuat satu EC2 worker langsung dan tidak membuat managed node group. Aplikasi memakai manifest Kubernetes yang sama, dengan overlay EKS untuk URL image. EKS dikenai biaya control plane meskipun worker memakai `t3.small`. [Harga EKS](https://aws.amazon.com/eks/pricing/).

## Kustomize, Argo CD, dan GitOps

`k8s/base` berisi manifest bersama. **Kustomize overlay** mengubah bagian yang berbeda per cluster: URL image dan pull secret untuk EC2/OpenShift, serta Route untuk OpenShift. `kubectl kustomize k8s/overlays/ec2` memperlihatkan hasil akhir tanpa memasang apa pun.

**GitOps** berarti Git menyimpan keadaan aplikasi yang diinginkan. GitHub Actions memperbarui tag image pada overlay setelah push ECR berhasil. **Argo CD** membaca repo Git secara berkala, membandingkan isi overlay dengan cluster, lalu menerapkan perubahan. Ia digunakan pada EKS melalui `gitops/application.yaml`. **OpenShift GitOps** adalah distribusi/operator Argo CD di OpenShift; ia membaca `gitops/application-openshift.yaml`. Secret ECR dikelola di luar Git karena berisi kredensial sementara.

```text
Commit kode → tes → build image → push ECR → commit tag SHA ke overlay
                                          ├→ Argo CD → EKS
                                          ├→ OpenShift GitOps → OpenShift
                                          └→ kubectl apply manual → kubeadm EC2
```

Argo CD dan OpenShift GitOps **tidak membangun image**; mereka hanya memasang image yang sudah ada di ECR. Terraform **tidak merilis aplikasi**; ia menyiapkan AWS. Ini membuat tanggung jawab tiap alat mudah dilihat saat ada kegagalan: cek job Actions untuk build/push, cek ECR untuk image, cek Git untuk tag, lalu cek status GitOps dan Pod untuk deploy.

## OpenShift, Route, dan monitoring

**OpenShift** adalah platform Kubernetes dari Red Hat. Pada use case ini, OpenShift Single Node berjalan pada EC2 yang dibuat installer resmi. OpenShift memerlukan resource jauh lebih besar daripada contoh kubeadm dan bukan bagian dari Free Tier. **Route** mengekspos Service aplikasi lewat HTTPS; pada manifest contoh, TLS berhenti di router OpenShift. [Panduan Single Node di AWS](https://docs.redhat.com/en/documentation/openshift_container_platform/4.22/html/installing_on_a_single_node/install-sno-installing-sno).

Untuk semua jalur, Kubernetes memeriksa `/healthz` agar Pod yang gagal dapat diketahui. Aplikasi juga menyediakan `/metrics` untuk Prometheus dan log melalui `kubectl logs` atau `oc logs`. Folder `monitoring/` berisi contoh konfigurasi Prometheus/Grafana, tetapi stack monitoring belum dipasang ke AWS oleh proyek ini. Pada EC2 `t3.small`, pasang monitoring di tempat lain agar tidak menghabiskan RAM node.
