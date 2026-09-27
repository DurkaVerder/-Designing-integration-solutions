import { render, screen } from '@testing-library/react';
import App from './App';

test('renders task manager login', () => {
  render(<App />);
  expect(screen.getByRole('heading', { name: 'Task manager' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument();
});
