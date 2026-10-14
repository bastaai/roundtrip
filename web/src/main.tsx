import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import './index.css';

// Inside the native macOS window there is no title bar: the header makes room
// for the window buttons and doubles as the drag area.
if (('runtime' in window || 'wails' in window || 'WailsInvoke' in window) && /Mac/.test(navigator.platform)) {
  document.documentElement.dataset.shell = 'mac';
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
