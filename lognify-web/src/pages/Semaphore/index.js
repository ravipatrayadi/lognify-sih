import React from "react";

const Semaphore = () => {
    React.useEffect(() => {
        window.open("https://ansible-dev.helo-k8s.fun/auth/login");
    }, []);

    return <h1>Redirected to SemaPhore</h1>;
};

export default Semaphore;
