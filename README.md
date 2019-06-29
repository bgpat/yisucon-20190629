# isucon-boilerplate

## usage

### initialize

Replace `$GITHUB_REPO_URL` and execute the following commands in the competition machine.  
(e.g., `GITHUB_REPO_URL=https://github.com/bgpat/isucon-boilerplate.git`)

```bash
cd /
git init
git remote add origin $GITHUB_REPO_URL
git fetch origin master
git reset --hard FETCH_HEAD
make USERS="bgpat fono09 Goryudyuma"
```

### edit config file

```bash
vim /etc/nginx/nginx.conf
```

### track source files by git

```bash
git add -f *.go
```
