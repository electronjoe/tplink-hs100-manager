# Robust Use of tp-link hs100 and hs110 Smart Outlets

This golang library leverages the excellent work of [jaedle/golang-tplink-hs100](https://github.com/jaedle/golang-tplink-hs100), extending it to support the next layer of tp-link hs100 / hs110 use: robust management of the devices.

Specifically, this library offers a Manager with features:

- Active monitoring for tp-link smartplug IP change
- Active monitoring for tp-link smartplug state change
    - E.g. user-actuated toggle, unplug/replug, etc
    - Via configurable polling
- Active re-assertion of tp-link smartplug desired state

These are the types of state / health issues that are going to be typical in arbitrary home network use of tp-link smartplugs, and tplink-hs100-manager is intended to address these issues.

## Design Decisions

### How to track identity

It's necessary to track identity of the smart plugs for discovery and state application.  The options here are to track by:

- Smartplug MAC address
- Smartplug IP address
- Smartplug tp-link configured name (via tp-link smartphone app)

Of these identifiers, only MAC address would necessarily be unique at all times.

Alternatively, if we use the tp-link configured name there is value in being able to logically map devices to purposes, without updating software configuration (e.g. via tp-link Kasa app for smartplug configuration).

### Explicitly call out intended management devices

There is value in providing the Manager explicit knowledge of which devices it is expected to be aware of. This allows warning logging or telemetry if the devices fall offline, and it can prevent mixed-management scenarios in the event that not all smartplugs are intended for control by the Manager.
