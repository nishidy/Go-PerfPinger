# Purpose
This is to briefly measure the bidirectional throughput on a device connected to multiple hosts by ping processes. Ping employs ICMP Echo and its reply to check if the target host is alive and measure RTT between them. This PerfPinger employs this by configuring the size and interval to measure the throughput using ICMP packet WITHOUT setting up the server process like iPerf, which needs run server at the remote host before starting the measurement.

# Usage
* Provide a file which contains target hosts
* Specify the file path, size in byte and interval in ms of ping
* Run with sudo because this uses priviledged raw ICMP endpoint

```
sudo ./perfpinger hosts 100 100
```

# Disclaimer
This generates neither TCP nor UDP while iPerf uses them. Thus, the handling of trasport layer is not considered in this measurement. Additionally, this aims to know how scarce the resource is in e.g. bad wireless environment and so this probably does not support the measurement for higher throughput. Thus, this is not for the case when you care about it and/or unidirectional measuremet is needed.

This is written in Go from scratch, but if you want to just use ping in shell, please see [this](https://gist.github.com/nishidy/a26d09ce5691daf8d4fe) in gist which actually works.

