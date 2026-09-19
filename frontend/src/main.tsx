
import React from 'react'; import ReactDOM from 'react-dom/client'; import { RouterProvider } from 'react-router-dom';
import { router } from './router'; import './styles.css';
import { AuthProvider } from './hooks/useAuth';
ReactDOM.createRoot(document.getElementById('root')!).render(<React.StrictMode><AuthProvider><RouterProvider router={router} future={{ v7_startTransition: true }}/></AuthProvider></React.StrictMode>);
