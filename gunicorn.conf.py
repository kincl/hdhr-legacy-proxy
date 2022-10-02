accesslog = "-"
access_log_format = '%(t)s %(h)s "%(r)s" %(s)s %(b)s "%(a)s"'
bind = "0.0.0.0"
worker_class = "gevent"
timeout = 900
