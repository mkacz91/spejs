import * as foo from './foo';
import * as empty_pb from 'proto/empty_pb';
import { JobServiceClient } from 'proto/JobServiceClientPb';
import * as universe_pb from 'proto/universe_pb';
import { UniverseServiceClient } from 'proto/UniverseServiceClientPb';
import * as THREE from 'three';
import { Matrix3, Matrix4 } from 'three';

let name: string = 'World';
foo.sayHello(name);
const client = new JobServiceClient('/rpc')
client.status(new empty_pb.Empty(), {}, (err, resp) => {
  console.log("err: ", err)
  console.log("resp: ", resp.toObject())
});
console.log(client)
console.log(empty_pb.Empty)

function main() {
  document.body.innerHTML = `
      <h1>Spejs</h1>
  
      <label for="coords-input">Coordinates:</label>
      <input type="text" id="coords-input">
  
      <button id="look-button">Look</button>
    `;

  const coordsInput = document.getElementById('coords-input') as HTMLInputElement;
  const lookButton = document.getElementById('look-button') as HTMLButtonElement;

  const universeService = new UniverseServiceClient('/rpc');

  lookButton.addEventListener('click', async () => {
    const coords = coordsInput.value.split(',').map(s => parseInt(s));
    if (coords.length !== 2 || isNaN(coords[0]!) || isNaN(coords[1]!)) {
      console.error(`Invalid coodrinate input "${coordsInput.value}"`);
      return;
    }

    const request = new universe_pb.OpticalSampleRequest();
    request.setX(coords[0]!);
    request.setY(coords[1]!);
    try {
      const response = await universeService.opticalSample(request);
      console.log(response.toObject());
    } catch (e) {
      console.error('===ERROR: ', e);
    }
  });

  const width = 800, height = 600;
  const renderer = new THREE.WebGLRenderer({ antialias: true });
  renderer.setSize(width, height);

  const camera = new THREE.PerspectiveCamera(70, width / height, 0.01, 10);
  camera.position.z = 1;
  const nearPyramid = new Matrix3(
    camera.aspect, 0, 0,
    0, 1, 0,
    0, 0, 1 / Math.tan(camera.fov / 360 * Math.PI),
  );

  const scene = new THREE.Scene();
  const env = createEnvMesh();
  scene.add(env);

  renderer.domElement.addEventListener('mousemove', e => {
    const r = (e.target as HTMLElement).getBoundingClientRect();
    const mx = e.offsetX / r.width;
    const my = e.offsetY / r.height;

    (env.material.uniforms.camera!.value as Matrix3)
      .setFromMatrix4(new Matrix4().makeRotationFromEuler(new THREE.Euler(
        (my - 0.5) * Math.PI,
        (mx - 0.5) * 2 * Math.PI,
        0,
        'YXZ',
      )))
      .multiply(nearPyramid);
  });

  renderer.setAnimationLoop(_ => {
    renderer.render(scene, camera);
  });

  document.body.appendChild(renderer.domElement);
}

const TEST_CUBE_MAP: THREE.CubeTexture = (() => {
  const c = document.createElement('canvas').getContext('2d')!;
  const faceSize = 32;
  c.canvas.width = faceSize;
  c.canvas.height = faceSize;
  c.font = `bold ${faceSize * 0.5}px monospace`;
  c.textAlign = 'center';
  c.textBaseline = 'middle';
  const faces = [
    ['#F00', '+X'],
    ['#F80', '-X'],
    ['#0A0', '+Y'],
    ['#8A0', '-Y'],
    ['#00F', '+Z'],
    ['#80F', '-Z'],
  ].map(([color, text]) => {
    c.fillStyle = color!;
    c.fillRect(0, 0, faceSize, faceSize);
    c.fillStyle = 'white';
    c.fillText(text!, faceSize / 2, faceSize / 2);
    return c.canvas.toDataURL();
  });
  return new THREE.CubeTextureLoader().load(faces);
})();

function createEnvMesh(): THREE.Mesh<THREE.BufferGeometry, THREE.RawShaderMaterial> {
  const geometry = new THREE.BufferGeometry()
  geometry.setAttribute('position', new THREE.BufferAttribute(new Float32Array([
    -1, -1, 1, 1, -1, 1,
    -1, -1, 1, -1, 1, 1,
  ]), 2));
  // Set bounding sphere to avoid it being computed fro position, which would
  // fail. Otherwise unused.
  geometry.boundingSphere = new THREE.Sphere();

  const material = new THREE.RawShaderMaterial({
    uniforms: {
      camera: { value: new Matrix3() },
      envMap: { value: TEST_CUBE_MAP },
    },
    vertexShader: `
      uniform mat3 camera;
      attribute vec2 position;
      varying vec3 envCoords;
      void main() {
        envCoords = camera * vec3(position, 1.0);
        gl_Position = vec4(position, 0.0, 1.0);
      }
    `,
    fragmentShader: `
      uniform samplerCube envMap;
      varying highp vec3 envCoords;
      void main() {
        gl_FragColor = textureCube(envMap, envCoords);
      }
    `,
    depthTest: false,
    depthWrite: false,
  });
  const mesh = new THREE.Mesh(geometry, material);
  mesh.frustumCulled = false;
  mesh.renderOrder = -1;
  return mesh;
}

main();