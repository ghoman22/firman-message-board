output "control_plane_public_ip" { value = aws_instance.node["control-plane"].public_ip }
output "control_plane_private_ip" { value = aws_instance.node["control-plane"].private_ip }
output "worker_public_ip" { value = aws_instance.node["worker"].public_ip }
output "worker_private_ip" { value = aws_instance.node["worker"].private_ip }
output "ssh_user" { value = "ubuntu" }
