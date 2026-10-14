import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import './index.css';
import { isDesktop } from './lib/shell';

// Inside the native macOS window there is no title bar: the header makes room
// for the window buttons and doubles as the drag area.
if (isDesktop && /Mac/.test(navigator.platform)) {
  document.documentElement.dataset.shell = 'mac';
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
