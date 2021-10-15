# Premium App Installation Guide

The Splunk Operator currently provides support for automating installation of Enterprise Security with support for other premium apps coming in the future. This page documents the prerequisites, installation steps, and limitations of deploying premium apps using the Splunk Operator. 


## Enterprise Security


### Prerequisites

Installing Enterprise Security in a Kubernetes cluster with the Splunk Operator requires the following:

* Ability to utilize the Splunk Operator [app framework](https://splunk.github.io/splunk-operator/AppFramework.html) method of installation.
* Access to the [Splunk Enterprise Security](https://splunkbase.splunk.com/app/263/) app package. 
* Pod resource specs that meet the [Enterprise Security hardware requirements](https://docs.splunk.com/Documentation/ES/latest/Install/DeploymentPlanning#Hardware_requirements).
* Enterprise Security Version 6.4.1 or 6.6.0 as Splunk Operator requires Splunk Enterprise 8.2.2 or later. For more information regarding Splunk Enterprise and Enterprise Security compatibility, see the [version compatibility matrix](https://docs.splunk.com/Documentation/VersionCompatibility/current/Matrix/CompatMatrix).



### Supported Deployment Types

Currently there are only a subset of architectures that support automated deployment of Enterprise Security through the Splunk Operator.

Supported Architectures Include:
* Standalone Splunk Instances 
* Standalone Search Head(s) which search any number of Indexer Clusters.
* Search Head Cluster(s) which search any number of Indexer Clusters. 

Notably, if deploying a distributed search environment, the use of indexer clustering is required to ensure that the necessary Enterprise Security specific configuration is pushed to the indexers via the Cluster Manager.

### What is automated by the Splunk Operator

The Splunk Operator will install the necessary Enterprise Security components depending on the architecture specified by the applied CRDs.

#### Standalone / Standalone Search Heads
For standalones and standalone search heads the Operator will install Splunk Enterprise Security and all associated domain add-ons (DAs), and supporting add-ons (SAs).

#### Search Head Cluster
When installing Enterprise Security in a Search Head Cluster, the Operator will stage ES and all associated DAs and SAs to the Deployer's etc/shcluster/apps directory, and will then push the apps to the Search Head Cluster members. This allows for an admin to [manage Enterprise Security through the deployer](https://docs.splunk.com/Documentation/ES/6.6.2/Install/InstallEnterpriseSecuritySHC#Managing_configuration_changes_in_a_search_head_cluster). 

#### Indexer Cluster
When installing ES on an indexer cluster the Splunk Operator will utilize the Cluster Manager to generate and distribute the [Splunk_TA_ForIndexers](https://docs.splunk.com/Documentation/ES/latest/Install/InstallTechnologyAdd-ons#Create_the_Splunk_TA_ForIndexers_and_manage_deployment_manually) app to the indexer cluster members. This TA contains indexes, props, and transforms configurations that are necessary for the indexers.


### How to Install Enterprise Security using the Splunk Operator


#### Necessary Configuration

When crafting your Custom Resource to create a Splunk Enterprise Deployment it is necessary to take the following configurations into account.

##### [appSources](https://splunk.github.io/splunk-operator/AppFramework.html#appsources) scope
   
   - When deploying ES to a Standalone or Standalone Search Head, it must be configured with an appSources scope of local.
   - When deploying ES to a Search Head Cluster or Indexer Cluster, it must be configured with an appSources scope of clusterWithPreConfig.

##### livenessInitialDelaySeconds 
As Splunk Enterprise Security is a large app package, it may be necessary to increase the livenessInitialDelaySeconds to allow sufficient time for the apps to be installed.  

The default livenessInitialDelaySeconds when utiltizing the App Framework method of installation is 1800 seconds, which may be large enough to install ES alone, however, if installing apps in conjunction with ES it will likely need to be raised to a higher value.

#####  SSL Enablement

When installing ES versions 6.3.0+ it is necessary to supply a value for the parameter ssl_enablement. By default the value of strict is used which requires Splunk to have SSL enabled in web.conf. The below table can be used for reference of available values. 

| SSL mode	| Description |
| --------- | ----------- |
|strict     |	Default mode. Ensure that SSL is enabled in the web.conf configuration file to use this mode. Otherwise, the installer exists with an error. |
| auto	   | Enables SSL in the etc/system/local/web.conf configuration file. |
| ignore	   | Ignores whether SSL is enabled or disabled. |

The Operator passes the ssl_enablement parameter through an ansible environment variable named "SPLUNK_ESS_SSL_ENABLEMENT" using the Operator's extraEnv feature.

```yaml
  extraEnv:
  - name: SPLUNK_ES_SSL_ENABLEMENT
    value: ignore
```

##### Search Head Cluster server.conf timeouts

It may be necessary to increase the value of the default Search Head Clustering network timeouts to ensure that the connections made from the deployer to the Search Heads while pushing apps do not timeout. 

These timeouts can be set through defaults.yaml
```yaml
  defaults: |-
    splunk:
      conf:
        - key: server
          value:
            directory: /opt/splunk/etc/system/local
              shclustering:
                rcv_timeout: 300
                send_timeeout: 300
                cxn_timeeout: 300
```

##### splunkdConnectionTimeout
Increasing the value of splunkdConnectionTimeout in web.conf will help ensure that all API calls made by the installer script will not timeout and prevent installation from succeeding.
```yaml
  defaults: |-
    splunk:
      conf:
        - key: web
          value:
            directory: /opt/splunk/etc/system/local
            content:
              settings:
                splunkdConnectionTimeout: 300
```


### Example YAML

The below yaml will configure ES on a Search Head Cluster which searches an Indexer Cluster. The assumptions made are that the ES app tarball exists in an s3 bucket location named "testfolder".
```yaml
apiVersion: enterprise.splunk.com/v2
kind: SearchHeadCluster
metadata:
  name: es-shc
  finalizers:
  - enterprise.splunk.com/delete-pvc
spec:
  appRepo:
    appSources:
    - location: testfolder
      name: appframework1
      scope: clusterWithPreConfig
      volumeName: volname
    appsRepoPollIntervalSeconds: 60
    defaults:
      volumeName: volname
    volumes:
    - endpoint: https://s3-us-west-2.amazonaws.com
      name: volname
      path: appbucket
      provider: aws
      secretRef: s3-secret
      storageType: s3
  livenessInitialDelaySeconds: 1800
  clusterMasterRef:
    name: es-cm
  defaults: |-
    splunk:
      conf:
        - key: server
          value:
            directory: /opt/splunk/etc/system/local
              shclustering:
                rcv_timeout: 300
                send_timeeout: 300
                cxn_timeeout: 300
        - key: web
          value:
            directory: /opt/splunk/etc/system/local
            content:
              settings:
                splunkdConnectionTimeout: 300
  extraEnv:
  - name: SPLUNK_ES_SSL_ENABLEMENT
    value: ignore
---
apiVersion: enterprise.splunk.com/v2
kind: ClusterMaster
metadata:
  name: es-cm
  finalizers:
  - enterprise.splunk.com/delete-pvc
spec:
  appRepo:
    appSources:
    - location: testfolder
      name: appframework1
      scope: clusterWithPreConfig
      volumeName: volname
    appsRepoPollIntervalSeconds: 60
    defaults:
      volumeName: volname
    volumes:
    - endpoint: https://s3-us-west-2.amazonaws.com
      name: volname
      path: appbucket
      provider: aws
      secretRef: s3-secret
      storageType: s3
  livenessInitialDelaySeconds: 1800
  defaults: |-
    splunk:
      conf:
        - key: web
          value:
            directory: /opt/splunk/etc/system/local
            content:
              settings:
                splunkdConnectionTimeout: 300
  extraEnv:
  - name: SPLUNK_ES_SSL_ENABLEMENT
    value: ignore
---
apiVersion: enterprise.splunk.com/v2
kind: IndexerCluster
metadata:
  name: es-idc
  finalizers:
  - enterprise.splunk.com/delete-pvc
spec:
  livenessInitialDelaySeconds: 1500
  clusterMasterRef:
    name: es-cm
  replicas: 3
```


#### Installation steps

All that needs to be done to install Enterprise Security through the Operator is to ensure that the Enterprise Security App package is present in the bucket configured through [AppFramework](https://splunk.github.io/splunk-operator/AppFramework.html) and to apply the specified custom resource(s). 

**Important Considerations**
* Installation may take upwards of 30 minutes for Search Head Clustering and Indexer Clustering environments.

* In indexer clustering environments, it is necessary to also install ES on the Cluster Manager to ensure the proper configurations are pushed to the indexer cluster members.


#### Post Installation Configuration

After installing Enterprise Security 

* [Deploy add-ons to Splunk Enterprise Security](https://docs.splunk.com/Documentation/ES/latest/Install/InstallTechnologyAdd-ons)

* [Setup Integration with Splunk Stream](https://docs.splunk.com/Documentation/ES/latest/Install/IntegrateSplunkStream) (optional)

* [Configure and deploy indexes](https://docs.splunk.com/Documentation/ES/latest/Install/Indexes)
  The indexes associated with the packaged DAs and SAs will automatically be pushed to indexers when using indexer clustering, this step is only necessary if it is desired to configure any [custom index configuration](https://docs.splunk.com/Documentation/ES/latest/Install/Indexes#Index_configuration). Additionally, any newly installed technical ad-ons which are not included with the ES package may require index deployment.

* [Configure Users and Roles as desired](https://docs.splunk.com/Documentation/ES/latest/Install/ConfigureUsersRoles)

* [Configure Datamodels](https://docs.splunk.com/Documentation/ES/latest/Install/Datamodels)


### Upgrade Steps

To upgrade ES, all that is required is to move the new ES package into the specified AppFramework bucket. This will initiate a pod reset and begin the process of upgrading the new version.

* The upgrade process will preserve any knowledge objects that exist in app local directories.

* Be sure to check the [ES upgrade notes](https://docs.splunk.com/Documentation/ES/latest/Install/Upgradetonewerversion#Version-specific_upgrade_notes) for any version specific changes.

### Troubleshooting


### Current Limitations

* Need to deploy add-ons to forwarders manually (or through your own methods).

* Need to deploy Stream App Manually
