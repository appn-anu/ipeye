# ipeye
golang application to capture images from ip cameras, designed to work within a docker container.

the following environment variables are available (some are configurable 

URL (required)
this is the url to grab an image from, this should return

OUTPUT=/data/MyCameraData (optional)
directory to output to on the docker container, (like if you have 10 of them and mount the same directory as a volume to /data)

NAME=MyCamera (optional)
the prefix for all the files, if not provided will just use the hostname

INTERVAL=10m (optional)
interval like 30s or 3m or 10m.
interval will be truncated to the last one.

TZ=Australia/Canberra (required)
timezone, this is for image file naming

IMAGETYPE=jpeg (optional)
only jpeg and tiff are available, tiff is default.
this will also govern the extension if it is .jpeg/.JPG etc.

TELEGRAF_HOST=(optional)
what the telegraf hostname is for a *udp socket listener*.
eg if you have telegraf running in docker it defaults to "telegraf:8092"


EXTRA_TAGS=tag1=something,tag2=somethingelse
give the telgraf metrics extra tags.
