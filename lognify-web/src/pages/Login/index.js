import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import "./Login.css";
const Login = () => {
    const navigate = useNavigate();
    const [formData, setFormData] = useState({
        uname: "",
        password: "",
    });

    const handleSubmit = (e) => {
        e.preventDefault();
        localStorage.setItem("username", formData.uname);
        localStorage.setItem("password", formData.password);
        navigate("/home");
    };

    return (
        <section
            className="login-container"
            style={{ backgroundColor: "white" }}
        >
            <div className="login-form">
                <img
                    src="https://www.aicte-india.org/sites/default/files/logo_new.png"
                    alt="Logo"
                    className="logo"
                />
                <h2>SIH</h2>
                <form method="post" onSubmit={handleSubmit}>
                    <div role="alert"></div>
                    <div>
                        <label>Username</label>
                        <div>
                            <input
                                required="true"
                                type="text"
                                name="text"
                                class="input"
                                onChange={(event) =>
                                    setFormData({
                                        ...formData,
                                        uname: event.target.value,
                                    })
                                }
                            />
                        </div>
                        <span className="text-danger"></span>
                    </div>
                    <div>
                        <label htmlFor="">Password</label>
                        <div>
                            <input
                                required="true"
                                type="password"
                                name="text"
                                class="input"
                                onChange={(event) =>
                                    setFormData({
                                        ...formData,
                                        password: event.target.value,
                                    })
                                }
                            />
                        </div>
                    </div>
                    <br />
                    <br />
                    <button type="submit">Log In</button>
                </form>
            </div>
        </section>
    );
};

export default Login;
