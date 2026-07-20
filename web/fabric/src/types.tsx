import React from 'react';

export type NavigationItem = {
  id: string;
  label: string;
  icon: React.ElementType;
};

export type TokenData = {
  date: string;
  input: number;
  output: number;
  total: number;
};
