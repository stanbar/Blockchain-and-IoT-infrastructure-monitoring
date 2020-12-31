export const repeat = (n: number) => (f: Function) => {
  if (n > 0) {
    f();
    repeat(n - 1)(f);
  }
};

export const generate = (n: number) => <T>(f: () => T): T[] => {
  return new Array(n).fill(undefined).map(f);
};
