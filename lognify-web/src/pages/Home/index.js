import React from 'react';
import { useNavigate } from 'react-router-dom';

const Home = () => {
  const navigate = useNavigate();

  const handleGrafana = () => {
    navigate('/grafana');
  };

  const handleSemaphore = () => {
    navigate('/semaphore');
  };

  return (
    <div style={styles.pageContainer}>
      <div style={styles.container}>
        <h1 style={styles.heading}>Welcome to the Home Page</h1>
        <p style={styles.description}>Explore the available options:</p>

        <div style={styles.buttonContainer}>
          <button style={styles.button} onClick={handleGrafana}>
            Go to Grafana
          </button>
          <button style={styles.button} onClick={handleSemaphore}>
            Go to Semaphore
          </button>
        </div>
      </div>
    </div>
  );
};

const styles = {
  pageContainer: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    height: '100vh',
    backgroundColor: '#0e1218',
  },
  container: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '20px',
    textAlign: 'center',
    backgroundColor: '#0e1218', // Set the background color of the container
    borderRadius: '10px', // Add border-radius if desired
    color: '#fff', // Set text color to white
  },
  heading: {
    fontSize: '24px',
    marginBottom: '10px',
  },
  description: {
    fontSize: '16px',
    marginBottom: '20px',
  },
  buttonContainer: {
    display: 'flex',
    gap: '10px',
  },
  button: {
    padding: '10px',
    fontSize: '16px',
    fontWeight: 'bold',
    backgroundColor: '#3498db',
    color: '#fff',
    border: 'none',
    borderRadius: '5px',
    cursor: 'pointer',
    transition: 'background-color 0.3s ease',

    ':hover': {
      backgroundColor: '#2980b9',
    },
  },
};

export default Home;
