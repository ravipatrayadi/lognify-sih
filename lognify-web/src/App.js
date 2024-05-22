
import Login from './pages/Login';
import Home from './pages/Home' ; 
import { BrowserRouter, Route, Routes } from "react-router-dom";
import Grafana from './pages/Grafana/Grafana';
import Semaphore from './pages/Semaphore';
function App() {
  return (
    <>
      
			<BrowserRouter>
				<Routes>
							<>
							<Route path="/" element={<Login />} />
							<Route path = "/home" element= {<Home/>} />
							<Route path = "/grafana" element= {<Grafana/>} />
							<Route path = "/semaphore" element= {<Semaphore/>} />						
							</>
					
				</Routes>
			</BrowserRouter>
    </>
  );
}

export default App;
