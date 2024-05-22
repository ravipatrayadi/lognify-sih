use std::{collections::BTreeSet, collections::HashMap, ffi::OsStr, fs, path::Path};

use anyhow::{bail, Context, Result};
use once_cell::sync::Lazy;
use serde::Deserialize;
use serde_json::Value;

type ComponentMap = HashMap<String, Component>;

type FeatureSet = BTreeSet<String>;

macro_rules! mapping {
    ( $( $key:ident => $value:ident, )* ) => {
        HashMap::from([
            $( (stringify!($key), stringify!($value)), )*
        ])
    };
}


static SOURCE_FEATURE_MAP: Lazy<HashMap<&'static str, &'static str>> = Lazy::new(|| {
    mapping!(
        prometheus_scrape => prometheus,
        prometheus_remote_write => prometheus,
    )
});

static TRANSFORM_FEATURE_MAP: Lazy<HashMap<&'static str, &'static str>> = Lazy::new(|| mapping!());

static SINK_FEATURE_MAP: Lazy<HashMap<&'static str, &'static str>> = Lazy::new(|| {
    mapping!(
        gcp_pubsub => gcp,
        gcp_stackdriver_logs => gcp,
        gcp_stackdriver_metrics => gcp,
        prometheus_remote_write => prometheus,
        splunk_hec_logs => splunk_hec,
    )
});

#[derive(Deserialize)]
pub struct SingerConfig {
    api: Option<Value>,
    enterprise: Option<Value>,

    #[serde(default)]
    sources: ComponentMap,
    #[serde(default)]
    transforms: ComponentMap,
    #[serde(default)]
    sinks: ComponentMap,
}

#[derive(Deserialize)]
struct Component {
    r#type: String,
}

pub fn load_and_extract(filename: &Path) -> Result<Vec<String>> {
    let config =
        fs::read_to_string(filename).with_context(|| format!("failed to read {filename:?}"))?;

    let config: SingerConfig = match filename
        .extension()
        .and_then(OsStr::to_str)
        .map(str::to_lowercase)
        .as_deref()
    {
        None => bail!("Invalid filename {filename:?}, no extension"),
        Some("json") => serde_json::from_str(&config)?,
        Some("toml") => toml::from_str(&config)?,
        Some("yaml" | "yml") => serde_yaml::from_str(&config)?,
        Some(_) => bail!("Invalid filename {filename:?}, unknown extension"),
    };

    Ok(from_config(config))
}

pub fn from_config(config: SingerConfig) -> Vec<String> {
    let mut features = FeatureSet::default();
    add_option(&mut features, "api", &config.api);
    add_option(&mut features, "enterprise", &config.enterprise);

    get_features(
        &mut features,
        "sources",
        config.sources,
        &SOURCE_FEATURE_MAP,
    );
    get_features(
        &mut features,
        "transforms",
        config.transforms,
        &TRANSFORM_FEATURE_MAP,
    );
    get_features(&mut features, "sinks", config.sinks, &SINK_FEATURE_MAP);
    features.remove("transforms-log_to_metric");

    features.into_iter().collect()
}

fn add_option<T>(features: &mut FeatureSet, name: &str, field: &Option<T>) {
    if field.is_some() {
        features.insert(name.into());
    }
}

fn get_features(
    features: &mut FeatureSet,
    key: &str,
    section: ComponentMap,
    exceptions: &HashMap<&str, &str>,
) {
    features.extend(
        section
            .into_values()
            .map(|component| component.r#type)
            .map(|name| {
                exceptions
                    .get(name.as_str())
                    .map_or(name, ToString::to_string)
            })
            .map(|name| format!("{key}-{name}")),
    );
}
