package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"runtime"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	windowWidth  = 800
	windowHeight = 600
	windowTitle  = "3D Physics Simulation"
	timeStep     = 0.01
)

// Object represents a 3D object in the simulation
type Object struct {
	Position   mgl32.Vec3
	Velocity   mgl32.Vec3
	Mass       float32
	Forces     mgl32.Vec3
	Elasticity float32
}

// NewObject creates a new object with given position, velocity, mass, and elasticity
func NewObject(position, velocity mgl32.Vec3, mass, elasticity float32) *Object {
	return &Object{
		Position:   position,
		Velocity:   velocity,
		Mass:       mass,
		Forces:     mgl32.Vec3{0, 0, 0},
		Elasticity: elasticity,
	}
}

// AddForce adds a force to the object
func (o *Object) AddForce(force mgl32.Vec3) {
	o.Forces = o.Forces.Add(force)
}

// Update updates the object's position and velocity based on the forces applied and the time step
func (o *Object) Update(dt float32) {
	// Calculate acceleration
	acceleration := o.Forces.Mul(1.0 / o.Mass)
	// Update velocity
	o.Velocity = o.Velocity.Add(acceleration.Mul(dt))
	// Update position
	o.Position = o.Position.Add(o.Velocity.Mul(dt))
	// Reset forces
	o.Forces = mgl32.Vec3{0, 0, 0}

	// Check for collision with the ground (y = -1.0)
	if o.Position.Y() < -1.0 {
		o.Position[1] = -1.0
		o.Velocity[1] = -o.Velocity.Y() * o.Elasticity
	}
}

// ModelMatrix returns the transformation matrix for the object's position
func (o *Object) ModelMatrix() mgl32.Mat4 {
	return mgl32.Translate3D(o.Position.X(), o.Position.Y(), o.Position.Z())
}

func init() {
	runtime.LockOSThread()
}

func main() {
	if err := glfw.Init(); err != nil {
		log.Fatalln("failed to initialize glfw:", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	window, err := glfw.CreateWindow(windowWidth, windowHeight, windowTitle, nil, nil)
	if err != nil {
		log.Fatalln("failed to create window:", err)
	}
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		log.Fatalln("failed to initialize glow:", err)
	}

	version := gl.GoStr(gl.GetString(gl.VERSION))
	fmt.Println("OpenGL version", version)

	var vao uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)

	vertices := []float32{
		-0.5, -0.5, 0.0,
		0.5, -0.5, 0.0,
		0.0, 0.5, 0.0,
	}

	var vbo uint32
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 3*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)

	program, err := newProgram("shaders/vertex_shader.glsl", "shaders/fragment_shader.glsl")
	if err != nil {
		log.Fatalln(err)
	}
	gl.UseProgram(program)

	object := NewObject(mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 5, 0}, 1.0, 0.8)

	modelLoc := gl.GetUniformLocation(program, gl.Str("model\x00"))

	ticker := time.NewTicker(time.Duration(timeStep*1000) * time.Millisecond)
	for !window.ShouldClose() {
		<-ticker.C

		processInput(window)

		object.AddForce(mgl32.Vec3{0, -9.81 * object.Mass, 0}) // Gravity force
		object.Update(timeStep)

		gl.ClearColor(0.2, 0.3, 0.3, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		gl.UseProgram(program)
		gl.BindVertexArray(vao)

		modelMatrix := object.ModelMatrix()
		gl.UniformMatrix4fv(modelLoc, 1, false, &modelMatrix[0])

		gl.DrawArrays(gl.TRIANGLES, 0, 3)

		window.SwapBuffers()
		glfw.PollEvents()
	}

	gl.DeleteVertexArrays(1, &vao)
	gl.DeleteBuffers(1, &vbo)
}

func processInput(window *glfw.Window) {
	if window.GetKey(glfw.KeyEscape) == glfw.Press {
		window.SetShouldClose(true)
	}
}

func newProgram(vertexShaderPath, fragmentShaderPath string) (uint32, error) {
	vertexShaderSource, err := ioutil.ReadFile(vertexShaderPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read vertex shader: %v", err)
	}

	fragmentShaderSource, err := ioutil.ReadFile(fragmentShaderPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read fragment shader: %v", err)
	}

	vertexShader, err := compileShader(string(vertexShaderSource)+"\x00", gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	fragmentShader, err := compileShader(string(fragmentShaderSource)+"\x00", gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	program := gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetProgramInfoLog(program, logLength, nil, &log[0])
		return 0, fmt.Errorf("failed to link program: %s", log)
	}

	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetShaderInfoLog(shader, logLength, nil, &log[0])
		return 0, fmt.Errorf("failed to compile %v: %s", source, log)
	}

	return shader, nil
}
