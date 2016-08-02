FROM debian

MAINTAINER Chris Dragga <chris.dragga@netapp.com>

LABEL version="1.0" \
	description="NetApp's usage tracker for Kubernetes Persistent Volumes"

ARG KUBERNETES_MASTER_IP
ARG KUBERNETES_MASTER_PORT
ARG MYSQL_IP
ARG BIN=kubevoltracker
ENV KUBERNETES_MASTER=${KUBERNETES_MASTER_IP}:${KUBERNETES_MASTER_PORT:-8080} \
	BIN=${BIN} \
	MYSQL_IP=${MYSQL_IP}
#EXPOSE ${KUBERNETES_MASTER_PORT}
#Expose the MySQL port.
#EXPOSE 3306
COPY ${BIN} /usr/local/bin/kubevoltracker
CMD ["/usr/local/bin/kubevoltracker"]
