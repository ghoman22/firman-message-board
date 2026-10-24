# Firman Message Board: 3 use case di AWS

Proyek ini adalah API papan pesan sederhana untuk belajar container, Kubernetes, CI/CD, dan GitOps. API menyediakan `GET/POST /messages`, `GET /healthz`, dan `GET /metrics`. Pesan disimpan di memori Pod dan hilang saat Pod restart. Semua langkah deploy di bawah menuju **AWS**; laptop hanya dipakai menjalankan CLI.

| Use case | Cluster | Ukuran contoh | Instalasi cluster | Biaya |
|---|---|---|---|---|
| **1. Kubernetes di EC2** | `kubeadm`: 1 control plane + 1 worker | 2 × `t3.small`, masing-masing 30 GB gp3 | Terraform `infra/terraform-ec2-kubeadm` + setup manual | Cek kelayakan Free Tier; dua EC2 memakai kredit lebih cepat |
| **2. Kubernetes di EKS** | EKS + dua managed node EC2 | 2 × `t3.small` | Terraform `infra/terraform` | **Berbayar**, termasuk control plane EKS |
| **3. OpenShift di EC2** | OpenShift Container Platform Single Node | Minimal 4 vCPU, 16 GiB, 120 GB; disarankan kapasitas di atas minimum | Installer resmi Red Hat membuat EC2 dan resource AWS | **Berbayar**, bukan `t3.small` |

**Penting soal Free Tier:** AWS membedakan akun dibuat sebelum dan sejak **15 Juli 2025**. Untuk akun lama, tipe EC2 Free Tier yang umum ialah `t2.micro`/`t3.micro`; untuk akun baru, `t3.small` masuk daftar tipe yang memenuhi syarat tetapi penggunaan dibatasi kredit dan masa Free plan. Jadi `t3.small` **tidak otomatis gratis**. Periksa **Billing → Free Tier** dan jenis akun sebelum `apply`. EBS, public IPv4, transfer data, dan resource lain juga dapat memakai kredit atau menimbulkan tagihan. EKS mengenakan biaya control plane per jam di luar EC2; OpenShift Single Node jauh melebihi ukuran Free Tier. [AWS EC2 Free Tier](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-free-tier-usage.html), [AWS EKS pricing](https://aws.amazon.com/eks/pricing/), [syarat OpenShift Single Node](https://docs.redhat.com/en/documentation/openshift_container_platform/4.22/html/installing_on_a_single_node/preparing-to-install-sno).

## Arsitektur dan alur CI/CD

```text
GitHub main ──> GitHub Actions: Terraform validate + Go test + Docker smoke test
            └─> build image bertag commit SHA ──> ECR private (Jakarta)
                  ├─> overlay EC2 ──> kubectl apply ──> kubeadm di dua EC2
                  ├─> overlay OpenShift ──> OpenShift GitOps ──> OpenShift di EC2
                  └─> overlay EKS ──> Argo CD ──> EKS + EC2 nodes
```

Aplikasi ditulis dengan Go standard library; Dockerfile menghasilkan image Distroless non-root. `k8s/base` berisi Deployment dan Service. Pipeline `.github/workflows/ci.yaml` menguji pada pull request, lalu pada push ke `main` menerbitkan **satu image ke ECR**. Pipeline mengubah tag di ketiga overlay Git. Argo CD/OpenShift GitOps menyinkronkan perubahan ke cluster masing-masing. **Kubernetes EC2 memakai `kubectl apply` manual** agar lab berukuran minimum tidak dibebani Argo CD. Pipeline tidak menjalankan `terraform apply`.

ECR adalah registry private. Node EKS menarik image memakai IAM role node. Kubernetes manual di EC2 dan OpenShift memakai Secret `ecr-pull` yang dibuat oleh `scripts/refresh-ecr-pull-secret.py`; tokennya berlaku **12 jam** dan perlu diperbarui sebelum Pod baru menarik image. [Autentikasi ECR](https://docs.aws.amazon.com/AmazonECR/latest/userguide/registry_auth.html), [EKS node role](https://docs.aws.amazon.com/eks/latest/userguide/create-node-role.html). Penjelasan istilah dan alur tersedia di [docs/KONSEP.md](docs/KONSEP.md).

Monitoring aplikasi tersedia sebagai liveness/readiness probe, log Pod, dan endpoint Prometheus `/metrics` dengan anotasi scrape di Service. Proyek belum memasang Prometheus/Grafana ke cluster; dashboard contoh ada di `monitoring/` untuk dipakai jika tim sudah punya monitoring. Pada `t3.small`, hindari memasang stack monitoring penuh di node yang sama.

## 0. Siapkan repository dan akun

1. Buat repository GitHub **public**. Jadikan **isi folder `Firman` sebagai root repository**, supaya `.github/workflows/ci.yaml` terdeteksi.
2. Ganti `REPLACE_OWNER/REPLACE_REPO` pada `gitops/application.yaml` dan `gitops/application-openshift.yaml` dengan URL repo baru. URL harus menunjuk root repository.
3. Siapkan AWS CLI v2, Terraform, Python 3, `kubectl`; untuk OpenShift tambahkan `oc` dan `openshift-install` sesuai versi cluster. Masuk ke akun AWS dan periksa identitas dan Free Tier:

```bash
aws sts get-caller-identity
aws freetier get-account-plan-state
curl -fsS https://checkip.amazonaws.com
```

Perintah Free Tier dapat gagal jika identitas IAM tidak punya izin Billing; lihat AWS Billing Console. Gunakan satu AWS Region untuk latihan: `ap-southeast-3` (Jakarta), lalu pastikan layanan/kuota tersedia di akunmu. Atur [AWS Budget](https://docs.aws.amazon.com/cost-management/latest/userguide/budgets-managing-costs.html) sebelum memakai jalur berbayar.

4. Dari root repository, push ke `main`:

```bash
git init -b main
git add .
git status --short
git commit -m "Mulai Firman Message Board"
git remote add origin https://github.com/PEMILIK/REPO.git
git push -u origin main
```

Jika repository sudah punya Git, gunakan remote yang ada. Jangan commit `terraform.tfvars`, `*.tfstate`, kubeconfig, atau private key. Di GitHub **Settings → Actions → General → Workflow permissions**, izinkan workflow menulis ke repository agar bot bisa commit tag image. Push pertama akan menjalankan pemeriksaan, tetapi job `publish` akan berhenti sampai dua variable ECR disiapkan pada langkah berikutnya.

### Buat ECR untuk ketiga use case

Terraform registry berdiri sendiri; jalankan **sekali** sebelum memilih cluster. Ini membuat ECR private di Jakarta dan role GitHub Actions OIDC yang hanya boleh push ke repository tersebut. Masukkan nama repo GitHub ke `terraform.tfvars`:

```bash
cp infra/terraform-registry/terraform.tfvars.example infra/terraform-registry/terraform.tfvars
terraform -chdir=infra/terraform-registry init
terraform -chdir=infra/terraform-registry fmt -check
terraform -chdir=infra/terraform-registry validate
terraform -chdir=infra/terraform-registry plan
terraform -chdir=infra/terraform-registry apply
terraform -chdir=infra/terraform-registry output
```

Simpan output `ecr_repository_url` dan `github_actions_role_arn` sebagai GitHub **Settings → Secrets and variables → Actions → Variables** bernama `ECR_REPOSITORY_URL` dan `AWS_ECR_PUSH_ROLE_ARN`. Jalankan **Actions → CI and GitOps delivery → Run workflow** di branch `main`. Pastikan job `publish` berhasil, image bertag commit SHA tampak di ECR, dan ketiga file `k8s/overlays/*/kustomization.yaml` berisi URL ECR serta SHA, bukan `bootstrap`. Jalankan `git pull --ff-only` setelah bot commit. IAM operator yang membuat pull secret harus boleh `ecr:GetAuthorizationToken` (resource `*`) serta `ecr:BatchGetImage` dan `ecr:GetDownloadUrlForLayer` untuk repository ECR ini. Jika akun sudah punya GitHub OIDC provider, isi `existing_github_oidc_provider_arn` pada variable registry.

ECR menimbulkan biaya penyimpanan image sesuai pemakaian; lifecycle policy menyimpan maksimal 20 image. Hapus registry **terakhir** setelah semua cluster dan aplikasi dibersihkan. Jika kamu pernah menjalankan Terraform versi lama yang membuat ECR di `infra/terraform`, pindahkan resource ke state registry dengan prosedur migrasi state sebelum `apply`; jangan membuat repository bernama sama dua kali atau langsung `destroy` state lama.

## 1. Kubernetes manual di EC2: 1 control plane + 1 worker

Jalur ini memakai **dua EC2 `t3.small`**: satu control plane (sering disebut master) dan satu worker. Terraform membuat VPC, subnet publik, Internet Gateway, security group, key pair, dua EC2 Ubuntu 24.04, dan dua volume gp3 30 GB. Port SSH hanya terbuka dari IP admin; komunikasi Kubernetes antarnode hanya diizinkan antar-EC2 dalam security group yang sama. CPU credits T3 disetel ke `standard`. Kubernetes dipasang **manual dengan `kubeadm`**, `containerd`, dan Flannel. Panduan resmi mensyaratkan minimal 2 GB RAM per node dan 2 CPU pada control plane, jadi `t3.small` berada tepat di batas minimum lab. Dua EC2, dua volume, dan dua public IPv4 memakai Free Tier/kredit lebih cepat daripada satu mesin. [Syarat kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/), [port Kubernetes](https://kubernetes.io/docs/reference/networking/ports-and-protocols/).

Di Kubernetes manual, istilahnya **worker node**. **Managed node group** adalah fitur pada use case EKS. Satu control plane ini tidak menyediakan high availability; jika EC2 control plane mati, API Kubernetes berhenti sampai mesin pulih.

Buat SSH key jika belum ada, lalu isi `admin_cidr` dengan IP publik hasil `curl` pada bagian persiapan:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519
cp infra/terraform-ec2-kubeadm/terraform.tfvars.example infra/terraform-ec2-kubeadm/terraform.tfvars
terraform -chdir=infra/terraform-ec2-kubeadm init
terraform -chdir=infra/terraform-ec2-kubeadm fmt -check
terraform -chdir=infra/terraform-ec2-kubeadm validate
terraform -chdir=infra/terraform-ec2-kubeadm plan
terraform -chdir=infra/terraform-ec2-kubeadm apply
terraform -chdir=infra/terraform-ec2-kubeadm output
```

Catat `control_plane_public_ip`, `control_plane_private_ip`, dan `worker_public_ip`. Ganti `IP_MASTER_PUBLIK`, `IP_MASTER_PRIVAT`, dan `IP_WORKER_PUBLIK` pada perintah berikut. Tunggu cloud-init selesai, lalu instal runtime dan paket Kubernetes **pada kedua node**. Script memakai repository paket Kubernetes v1.36 dan mengunci paket agar tidak naik versi tanpa prosedur upgrade:

```bash
ssh -i ~/.ssh/id_ed25519 ubuntu@IP_MASTER_PUBLIK 'sudo cloud-init status --wait'
ssh -i ~/.ssh/id_ed25519 ubuntu@IP_WORKER_PUBLIK 'sudo cloud-init status --wait'
ssh -i ~/.ssh/id_ed25519 ubuntu@IP_MASTER_PUBLIK 'sudo bash -s' < scripts/install-kubeadm-node.sh
ssh -i ~/.ssh/id_ed25519 ubuntu@IP_WORKER_PUBLIK 'sudo bash -s' < scripts/install-kubeadm-node.sh
```

Buat cluster pada control plane. `IP_MASTER_PRIVAT` adalah output Terraform, bukan IP publik. Flannel memakai Pod CIDR `10.244.0.0/16`; node di VPC memakai `10.43.0.0/16`, sehingga kedua rentang tidak bertabrakan. [Dokumentasi Flannel](https://github.com/flannel-io/flannel/blob/master/Documentation/kubernetes.md).

```bash
ssh -i ~/.ssh/id_ed25519 ubuntu@IP_MASTER_PUBLIK 'sudo kubeadm init --apiserver-advertise-address=IP_MASTER_PRIVAT --apiserver-cert-extra-sans=127.0.0.1 --pod-network-cidr=10.244.0.0/16 --cri-socket=unix:///run/containerd/containerd.sock'
ssh -i ~/.ssh/id_ed25519 ubuntu@IP_MASTER_PUBLIK 'sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml'
```

Gabungkan worker dengan token sementara dari control plane; token mengalir lewat SSH dan tidak ditulis ke Git:

```bash
ssh -i ~/.ssh/id_ed25519 ubuntu@IP_MASTER_PUBLIK 'sudo kubeadm token create --print-join-command' | ssh -i ~/.ssh/id_ed25519 ubuntu@IP_WORKER_PUBLIK 'sudo bash -s'
ssh -i ~/.ssh/id_ed25519 ubuntu@IP_MASTER_PUBLIK 'sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes -o wide'
```

Tunggu kedua node berstatus `Ready`. Control plane tetap bertaint dan aplikasi dijadwalkan pada worker. Untuk mengakses API tanpa membuka port 6443 ke internet, jalankan SSH tunnel ini pada terminal pertama dan biarkan aktif:

```bash
ssh -i ~/.ssh/id_ed25519 -N -L 6443:127.0.0.1:6443 ubuntu@IP_MASTER_PUBLIK
```

Pada terminal kedua, ambil kubeconfig. Perintah Python mengarahkan alamat API di kubeconfig ke ujung tunnel; sertifikat API sudah memuat SAN `127.0.0.1` dari langkah `kubeadm init`:

```bash
ssh -i ~/.ssh/id_ed25519 ubuntu@IP_MASTER_PUBLIK 'sudo cat /etc/kubernetes/admin.conf' > ec2-kubeconfig.yaml
chmod 600 ec2-kubeconfig.yaml
python3 -c "from pathlib import Path; import re; p=Path('ec2-kubeconfig.yaml'); p.write_text(re.sub(r'server: https://[^:]+:6443', 'server: https://127.0.0.1:6443', p.read_text()))"
KUBECONFIG=./ec2-kubeconfig.yaml kubectl get nodes
KUBECONFIG=./ec2-kubeconfig.yaml kubectl apply -k k8s/namespace
KUBECONFIG=./ec2-kubeconfig.yaml python3 scripts/refresh-ecr-pull-secret.py --client kubectl --repository-url URL_ECR
KUBECONFIG=./ec2-kubeconfig.yaml kubectl apply -k k8s/overlays/ec2
KUBECONFIG=./ec2-kubeconfig.yaml kubectl -n firman-demo rollout status deployment/message-board --timeout=300s
KUBECONFIG=./ec2-kubeconfig.yaml kubectl -n firman-demo port-forward service/message-board 8080:80
```

Ganti `URL_ECR` dengan output `ecr_repository_url` dari Terraform registry. Port-forward berjalan terus pada terminal kedua; di terminal ketiga uji aplikasi:

```bash
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS -X POST http://127.0.0.1:8080/messages -H 'Content-Type: application/json' -d '{"text":"Halo dari kubeadm EC2"}'
curl -fsS http://127.0.0.1:8080/messages
```

Setiap image baru masuk ECR dan overlay EC2 diperbarui oleh pipeline; jalankan `git pull --ff-only`, perbarui pull secret, lalu `kubectl apply -k k8s/overlays/ec2` dengan `KUBECONFIG` yang sama. Token pull ECR berlaku 12 jam. Setelah latihan, tutup tunnel, jalankan `terraform -chdir=infra/terraform-ec2-kubeadm destroy`, lalu hapus `ec2-kubeconfig.yaml` dari komputer. Jangan hapus state Terraform sebelum destroy berhasil. Jika sudah pernah membuat EC2 K3s versi lama, bersihkan cluster dan state lama dahulu sebelum memakai konfigurasi dua node ini.

## 2. Kubernetes di Amazon EKS (dua EC2 `t3.small`)

Terraform membuat VPC, EKS, dan dua managed node `t3.small`. ECR serta role push sudah dibuat pada langkah registry bersama. Node berada di subnet publik dengan public IPv4 supaya dapat menarik image tanpa NAT Gateway; control plane memakai subnet privat. Endpoint API EKS dibatasi ke IP admin. Ukuran dua node ini cukup untuk lab kecil, tetapi Argo CD dan addon EKS dapat menghabiskan ruang; jika Pod `Pending` karena memori, gunakan instance lebih besar dan periksa biaya. EKS control plane, dua EC2, EBS, dan public IPv4 **tetap berbayar**. [EKS networking](https://docs.aws.amazon.com/eks/latest/userguide/network-reqs.html), [EKS pricing](https://aws.amazon.com/eks/pricing/).

```bash
cp infra/terraform/terraform.tfvars.example infra/terraform/terraform.tfvars
```

Isi `cluster_public_access_cidrs` dengan `IP_KAMU/32` dan `aws_region` Jakarta. Kemudian:

```bash
terraform -chdir=infra/terraform init
terraform -chdir=infra/terraform fmt -check
terraform -chdir=infra/terraform validate
terraform -chdir=infra/terraform plan
terraform -chdir=infra/terraform apply
terraform -chdir=infra/terraform output
aws eks update-kubeconfig --region ap-southeast-3 --name NAMA_CLUSTER
kubectl get nodes
```

Ganti `NAMA_CLUSTER` dari output Terraform. Registry dan image ECR seharusnya sudah siap dari langkah bersama. Periksa `k8s/overlays/prod/kustomization.yaml`: URL ECR dan tag SHA harus terisi. Managed node EKS memakai izin tarik ECR pada IAM role node. Lalu pasang Argo CD:

```bash
kubectl create namespace argocd
kubectl apply -n argocd --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl -n argocd rollout status deployment/argocd-server --timeout=300s
kubectl apply -f gitops/application.yaml
kubectl -n firman-demo rollout status deployment/message-board --timeout=300s
kubectl -n firman-demo port-forward service/message-board 8080:80
```

Jika namespace sudah ada, lewati `create namespace`. Di terminal lain jalankan tiga perintah `curl` dari use case 1. Argo CD membaca perubahan tag dari Git setelah setiap rilis. Untuk membersihkan:

```bash
kubectl -n argocd delete application message-board --ignore-not-found
terraform -chdir=infra/terraform plan -destroy
terraform -chdir=infra/terraform destroy
```

ECR tetap ada karena state Terraform registry terpisah; hapus setelah semua use case selesai.

## 3. OpenShift Single Node di EC2

Ini jalur untuk belajar fitur OpenShift seperti **Route** dan **OpenShift GitOps**. OpenShift Single Node di AWS dipasang memakai **installer resmi dengan `platform=aws`**, yang membuat EC2 beserta infrastruktur pendukung; sekadar memasang paket OpenShift ke Ubuntu EC2 tidak cukup. Red Hat menetapkan minimum **4 vCPU, 16 GiB RAM, 120 GB storage** dan menyebut 4 vCPU hanya memberi sedikit ruang untuk aplikasi. Siapkan instance dengan kapasitas di atas minimum (misalnya kelas 8 vCPU/32 GiB) sesuai dukungan installer dan kuota AWS, pull secret/subscription Red Hat, domain DNS, serta biaya AWS yang disetujui. Proyek ini **tidak mengotomasi pembuatan cluster OpenShift**, karena ukuran dan prasyaratnya berada di luar Free Tier. Ikuti [panduan resmi SNO di AWS](https://docs.redhat.com/en/documentation/openshift_container_platform/4.22/html/installing_on_a_single_node/install-sno-installing-sno) dan [persiapan SNO](https://docs.redhat.com/en/documentation/openshift_container_platform/4.22/html/installing_on_a_single_node/preparing-to-install-sno) untuk membuat cluster; simpan direktori instalasi dan kubeconfig untuk proses uninstall.

Alur installer: ikuti [bagian resmi SNO di AWS](https://docs.redhat.com/en/documentation/openshift_container_platform/4.22/html/installing_on_a_single_node/install-sno-installing-sno), lalu pada prosedur **Installing a cluster on AWS with customizations** pilih Region Jakarta, domain Route 53 yang dikuasai, pull secret Red Hat, satu control-plane node (`controlPlane.replicas: 1`), tanpa worker (`compute.replicas: 0`), dan tipe/volume EC2 yang memenuhi kapasitas di atas. Simpan seluruh direktori kerja installer di tempat aman di luar repository. Setelah instalasi selesai, gunakan kubeconfig yang dihasilkan installer; proses pembuatan cluster dan uninstall tetap mengikuti panduan Red Hat karena mencakup DNS, IAM, jaringan, dan resource AWS tambahan.

Setelah `oc get nodes` menunjukkan node `Ready`, instal **Red Hat OpenShift GitOps Operator** melalui OperatorHub sesuai [dokumentasi resminya](https://docs.redhat.com/en/documentation/red_hat_openshift_gitops/1.19/html/installing_gitops/installing-openshift-gitops). Pastikan overlay `k8s/overlays/openshift/kustomization.yaml` sudah berisi URL ECR dan tag SHA. Dari root repository, buat project dan pull secret ECR **sebelum** memasang aplikasi GitOps:

```bash
oc get nodes
oc -n openshift-gitops get pods
oc new-project firman-demo
oc label namespace firman-demo argocd.argoproj.io/managed-by=openshift-gitops
python3 scripts/refresh-ecr-pull-secret.py --client oc --repository-url URL_ECR
oc apply -f gitops/application-openshift.yaml
oc -n openshift-gitops get application message-board-openshift
oc -n firman-demo rollout status deployment/message-board --timeout=300s
oc -n firman-demo get route message-board
```

Ganti `URL_ECR` dengan output registry. Jika project sudah ada, lewati `oc new-project`. Perbarui secret dengan perintah yang sama sebelum token 12 jam kedaluwarsa, khususnya sebelum rollout baru. Gunakan host Route dari perintah terakhir:

```bash
curl -fsS https://HOST_ROUTE/healthz
curl -fsS -X POST https://HOST_ROUTE/messages -H 'Content-Type: application/json' -d '{"text":"Halo dari OpenShift EC2"}'
```

Setelah latihan, hapus aplikasi dan project, lalu **uninstall cluster memakai installer dan direktori instalasi yang sama**, sesuai panduan resmi Red Hat. Periksa AWS Console agar tidak ada EC2, volume, load balancer, atau resource lain yang tertinggal.

### Bersihkan ECR setelah semua use case selesai

Setelah cluster dan aplikasi yang memakai image ECR dihapus, periksa rencana penghapusan registry lalu jalankan:

```bash
terraform -chdir=infra/terraform-registry plan -destroy
terraform -chdir=infra/terraform-registry destroy
```

`force_delete` menghapus repository beserta image di dalamnya. Hapus dua GitHub Actions variables ECR setelah destroy. State Terraform disimpan terpisah di masing-masing folder `infra/`; jangan hapus state sebelum `destroy` selesai.

## Cek dan troubleshooting

| Gejala | Periksa |
|---|---|
| Pipeline tidak publish | Tiga job pemeriksaan harus lulus dan push berasal dari `main`. |
| Bot tidak bisa commit tag | Workflow permissions dan aturan proteksi branch `main`. |
| `ImagePullBackOff` pada EC2/OpenShift | URL/tag ECR, izin IAM operator, dan masa berlaku Secret `ecr-pull`; jalankan ulang helper. |
| `ImagePullBackOff` pada EKS | URL/tag ECR, izin node menarik image, dan jaringan node ke ECR. |
| Pod `Pending` | Memori/CPU node `t3.small`; jalankan `kubectl describe pod` atau `oc describe pod`. |
| Node kubeadm belum `Ready` | Cek `sudo journalctl -u kubelet -n 100`, status `containerd`, Flannel, dan apakah worker sudah `join`. |
| Tunnel EC2 gagal | IP control plane terbaru, SSH key, `admin_cidr`, dan port 6443 yang belum dipakai di komputer operator. |
| GitOps `OutOfSync` | URL repository, branch `main`, path overlay, dan label namespace OpenShift. |

Log aplikasi: `kubectl -n firman-demo logs deployment/message-board` atau `oc -n firman-demo logs deployment/message-board`. Metrik: `curl http://127.0.0.1:8080/metrics` saat port-forward aktif. Semua contoh aplikasi hanya memakai satu replica dan tanpa database.

## Peta file

| Path | Fungsi |
|---|---|
| `main.go`, `Dockerfile` | API dan image |
| `.github/workflows/ci.yaml`, `scripts/update-image.py` | CI/CD dan pembaruan tag |
| `infra/terraform-ec2-kubeadm/`, `scripts/install-kubeadm-node.sh` | Dua EC2 serta instalasi Kubernetes manual |
| `infra/terraform-registry/` | ECR private dan OIDC GitHub Actions untuk semua use case |
| `infra/terraform/` | EKS dan dua managed node |
| `k8s/base/` | Deployment dan Service bersama |
| `k8s/overlays/ec2/` | Image ECR dan pull secret untuk Kubernetes EC2 |
| `k8s/overlays/prod/`, `gitops/application.yaml` | Image ECR dan Argo CD EKS |
| `k8s/overlays/openshift/`, `gitops/application-openshift.yaml` | Image ECR, Route, dan OpenShift GitOps |
| `scripts/refresh-ecr-pull-secret.py` | Memperbarui kredensial pull ECR pada EC2/OpenShift |
| `docs/KONSEP.md` | Penjelasan singkat teknologi dan alur deploy |
| `monitoring/` | Contoh konfigurasi Prometheus/Grafana; belum dipasang di AWS |
