%global debug_package %{nil}
%global __strip /bin/true

Name:           caddy
Version:        %{ver}
Release:        %{rel}%{?dist}

Summary:	caddy is the HTTP/2 web server with automatic HTTPS.

Group:		SDS
License:	GPL
URL:		http://github.com/journeymidnight
Source0:	%{name}-%{version}-%{rel}.tar.gz
BuildRoot:	%(mktemp -ud %{_tmppath}/%{name}-%{version}-%{release}-XXXXXX)
#BuildRequires:  
#Requires:       libradosstriper1

%description

%prep
%setup -q -n %{name}-%{version}-%{rel}


%build
#The go build still use source code in GOPATH/src/legitlab/yig/
#keep git source tree clean, better ways to build?
#I do not know
make build


%install
rm -rf %{buildroot}
install -D -m 755 caddy/caddy %{buildroot}%{_bindir}/caddy
install -D -m 644 package/caddy.logrotate %{buildroot}/etc/logrotate.d/caddy.logrotate
install -D -m 644 package/caddy.service   %{buildroot}/usr/lib/systemd/system/caddy.service
install -D -m 644 conf/Caddyfile %{buildroot}%{_sysconfdir}/caddy/Caddyfile
install -D -m 644 conf/mime.types %{buildroot}%{_sysconfdir}/caddy/mime.types
install -D -m 644 conf/prometheus %{buildroot}%{_sysconfdir}/caddy/prometheus
install -d %{buildroot}/var/log/caddy/

#ceph confs ?

%post
systemctl enable caddy


%preun

%clean
rm -rf %{buildroot}

%files
%defattr(-,root,root,-)
%config(noreplace) /etc/caddy/Caddyfile
/usr/bin/caddy
/etc/logrotate.d/caddy.logrotate
%dir /var/log/caddy/
/usr/lib/systemd/system/caddy.service


%changelog
