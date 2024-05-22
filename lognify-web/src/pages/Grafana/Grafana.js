import React from "react";

const Grafana = () => {
    React.useEffect(() => {
        window.open("https://gf-dev.helo-k8s.fun/login");
    }, []);

    return <h1>Redirected to Grafana</h1>;
};

export default Grafana;
