# Homonculus
Part of terabiome ecosystem, used for provisioning and managing VMs in bare-metals with my who-knows-why convention specification files 😛

Mostly work out-of-the-box by using `sudo HOME=<enter your current user $HOME, since I mount SSH keys from host into container> bash scripts/container-<whatever>`, but it is up to your preferences.

APIs are to be consumed by orchestrators (e.g Temporal telling homonculus to create 1 VM with specific specs + series of cloud-init commands, or delete it, or bootstrapping k3s cluster, etc) or direct HTTP calls.

Have fun if you happen to use this 😃
