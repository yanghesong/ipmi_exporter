## Running
A minimal invocation looks like this:

    ./ipmi_exporter --config.file=ipmi_remote.yml 

## Get Gata
Deployment IP and port: 10.105.12.200:9290. target IP address to be monitored is 10.105.14.80. you can get the data by using the following command:

    curl http://10.105.12.200:9290/ipmi?target=10.105.14.80

The corresponding data returned:

    # HELP ipmi_scrape_duration_seconds Returns how long the scrape took to complete in seconds.
    # TYPE ipmi_scrape_duration_seconds gauge
    ipmi_scrape_duration_seconds 0.224413809
    # HELP ipmi_sel_gpu_leak_status Current Assertion Event for GPU Leak Status.
    # TYPE ipmi_sel_gpu_leak_status gauge
    ipmi_sel_gpu_leak_status 1
    # HELP ipmi_sel_mb_leak_status Current Assertion Event for MB Leak Status.
    # TYPE ipmi_sel_mb_leak_status gauge
    ipmi_sel_mb_leak_status 1
    # HELP ipmi_up '1' if a scrape of the IPMI device was successful, '0' otherwise.
    # TYPE ipmi_up gauge
    ipmi_up{collector="sel"} 1



