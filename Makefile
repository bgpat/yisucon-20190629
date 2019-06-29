USERS := bgpat fono09 Goryudyuma

define pubkey
	wget -q -O- https://github.com/$(1).keys \
	| sed 's/^/environment="GIT_AUTHOR_NAME=$(1)",environment="GIT_AUTHOR_EMAIL=$(1)@users.noreply.github.com" /' \
	>> /root/.ssh/authorized_keys

endef

.PHONY: all
all: git ssh
	@clear
	@echo "Open \e[4mhttps://github.com/$$(\
		git remote get-url origin | sed -r 's/^.*?:(.*)\.git$$/\1/' \
	)/settings/keys/new\e[0m and paste pubkey:\n"
	@cat /root/.ssh/id_rsa.pub
	@echo

.PHONY: git
git: /.gitignore /root/.vimrc /home/isucon/.vimrc /root/.gitconfig
	git remote get-url origin \
	| grep '^https://' \
	&& git remote get-url origin \
	| sed -r 's%^https://github.com/([a-zA-Z0-9_-]*/[a-zA-Z0-9_-]*).*$$%git@github.com:\1.git%' \
	| xargs git remote set-url origin \
	|| true

.PHONY: ssh
ssh: /root/.ssh/authorized_keys /root/.ssh/id_rsa sshd

.PHONY: sshd
sshd: /etc/sudoers /etc/ssh/sshd_config

.PHONY: /etc/sudoers
/etc/sudoers: /etc/sudoers.bak
	echo 'Defaults	env_keep+="GIT_AUTHOR_NAME"' >> $@
	echo 'Defaults	env_keep+="GIT_AUTHOR_EMAIL"' >> $@

/etc/sudoers.bak:
	cp -p /etc/sudoers $@

.PHONY: /etc/ssh/sshd_config
/etc/ssh/sshd_config: /etc/ssh/sshd_config.bak
	sed -i '/^PermitRootLogin no/d' $@
	echo 'PermitUserEnvironment yes' >> $@
	echo 'PermitRootLogin without-password' >> $@
	sshd -t
	systemctl reload sshd

/etc/ssh/sshd_config.bak:
	cp -p /etc/ssh/sshd_config $@

.PHONY: clean
clean:
	rm -rf /root/.ssh /.gitignore /root/.vimrc /home/isucon/.vimrc /root/.ssh/id_rsa /root/.gitconfig
	test -e /etc/sudoers.bak && cp -fp /etc/sudoers.bak /etc/sudoers
	test -e /etc/ssh/sshd_config.bak && cp -fp /etc/ssh/sshd_config.bak /etc/ssh/sshd_config

/root/.ssh:
	mkdir -p /root/.ssh
	chmod 700 /root/.ssh

/root/.ssh/authorized_keys: /root/.ssh
	$(foreach user,$(USERS),$(call pubkey,$(user)))
	chmod 600 /root/.ssh/authorized_keys

/.gitignore: files/gitignore
	cp -f $< $@

/root/.vimrc: files/vimrc
	cp -f $< $@

/home/isucon/.vimrc: files/vimrc
	cp -f $< $@

/root/.ssh/id_rsa: /root/.ssh
	yes | ssh-keygen -f $@ -t rsa -N "" -b 4096 > /dev/null

/root/.gitconfig:
	git config --global user.email "anonymous@example.com"
	git config --global user.name "anonymous"
