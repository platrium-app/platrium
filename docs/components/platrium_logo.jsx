import * as React from "react";
const PlatriumLogo = (props) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width={512}
    height={512}
    viewBox="0 0 512 512"
    {...props}
  >
    <filter id="a" x="-50%" y="-50%" width="200%" height="200%">
      <feOffset in="blur" result="offsetBlur" />
      <feFlood floodColor="#fff" floodOpacity={0.9} result="offsetColor" />
      <feComposite
        in="offsetColor"
        in2="offsetBlur"
        operator="in"
        result="offsetBlur"
      />
      <feMerge>
        <feMergeNode in="offsetBlur" />
        <feMergeNode in="SourceGraphic" />
      </feMerge>
    </filter>
    <path
      d="M256 75a181 181 0 1 0 181 181H324a68 68 0 1 1-68-68z"
      fill="#06f"
      stroke="#fff"
      strokeWidth={8}
      filter="url(#a)"
    />
    <path
      d="M256 60v128a68 68 0 0 1 68 68h128A196 196 0 0 0 256 60Z"
      fill="#ed1a38"
      stroke="#fff"
      strokeWidth={8}
      filter="url(#a)"
    />
  </svg>
);

export default PlatriumLogo;
